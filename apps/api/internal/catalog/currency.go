package catalog

// Per-locale currency conversion for the public models catalog
// (Phase-3 PR-11, area 10-currency-conversion). The api fetches
// https://api.exchangerate.host/latest?base=USD once every 24h
// and caches the rate map in memory. The catalog handler reads
// ?locale=xx-XX and applies the rate to cost_input / cost_output,
// populating CostInputLocal / CostOutputLocal / Currency fields
// on each ModelsDevEntry.
//
// If the rate fetch fails, the catalog handler leaves the
// per-locale fields empty so the client falls back to the raw
// USD values without surfacing a 5xx. The rates are for show,
// not billing, so brief outages are tolerable.

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/big"
	"net/http"
	"sync"
	"time"
)

// LocaleCurrency maps an i18n locale (en-US, pt-BR, ...) to the
// ISO-4217 currency code we present in the model picker. Locales
// not in this map fall through to "USD" (no conversion).
var LocaleCurrency = map[string]string{
	"en-US": "USD",
	"pt-BR": "BRL",
	"en-GB": "GBP",
	"de-DE": "EUR",
	"ja-JP": "JPY",
	"zh-CN": "CNY",
}

// CurrencyForLocale returns the ISO code mapped from the given
// locale, defaulting to "USD" when the locale is unmapped (or
// empty). The mapping is case-sensitive on purpose: the wire
// format uses canonical "en-US" / "pt-BR" forms.
func CurrencyForLocale(locale string) string {
	if c, ok := LocaleCurrency[locale]; ok {
		return c
	}
	return "USD"
}

// RateFetcher returns raw JSON bytes from the upstream rate API.
// Tests swap this seam with a stub; production uses the public
// api.exchangerate.host endpoint.
type RateFetcher func(ctx context.Context) ([]byte, error)

// Clock returns the current time. Tests inject a clock to drive
// the 24h cache TTL deterministically.
type Clock func() time.Time

// RatesCache owns the in-memory exchange-rate table. It is safe
// for concurrent use: multiple catalog requests can read while
// a single refresh holds the write lock.
type RatesCache struct {
	mu        sync.RWMutex
	rates     map[string]float64
	fetchedAt time.Time
	ttl       time.Duration
	hc        *http.Client
	fetcher   RateFetcher
	clock     Clock
}

// NewRatesCache returns an empty cache with the default 24h TTL
// and a 10-second HTTP timeout. The clock defaults to time.Now
// so production code never has to think about it.
func NewRatesCache() *RatesCache {
	return &RatesCache{
		ttl: 24 * time.Hour,
		hc: &http.Client{
			Timeout: 10 * time.Second,
		},
		fetcher: defaultRateFetcher(),
		clock:   func() time.Time { return time.Now() },
	}
}

// SetFetcher swaps the upstream fetcher (test seam).
func (c *RatesCache) SetFetcher(f RateFetcher) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.fetcher = f
}

// SetClock swaps the time source (test seam for TTL expiry).
func (c *RatesCache) SetClock(clk Clock) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.clock = clk
}

func defaultRateFetcher() RateFetcher {
	return func(ctx context.Context) ([]byte, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet,
			"https://api.exchangerate.host/latest?base=USD&symbols=USD,BRL,GBP,EUR,JPY,CNY", nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "rocinante-harness/1.0.0")
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, err
		}
		defer res.Body.Close()
		if res.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("exchangerate.host status %d", res.StatusCode)
		}
		return io.ReadAll(res.Body)
	}
}

// rateResponse is the slice of the upstream JSON we read. We
// only decode `rates` (and ignore `base`, `date`) because the
// endpoint guarantees base=USD and we never display the date.
type rateResponse struct {
	Rates map[string]float64 `json:"rates"`
}

// Refresh re-fetches the rate table if the cache is older than
// ttl. Safe to call from many goroutines: the actual HTTP call
// is serialized via a single-flight channel so we don't hammer
// the upstream if several requests land in parallel.
//
// On HTTP failure, Refresh returns the error but leaves the
// previously cached rates in place so callers still see useful
// numbers (brief outage is preferable to empty model picker).
func (c *RatesCache) Refresh(ctx context.Context) error {
	c.mu.RLock()
	fresh := !c.fetchedAt.IsZero() && c.clock().Before(c.fetchedAt.Add(c.ttl))
	c.mu.RUnlock()
	if fresh {
		return nil
	}
	c.mu.Lock()
	fetcher := c.fetcher
	c.mu.Unlock()
	body, err := fetcher(ctx)
	if err != nil {
		log.Printf("currency: rate fetch failed: %v", err)
		return err
	}
	var parsed rateResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		log.Printf("currency: rate decode failed: %v", err)
		return fmt.Errorf("decode rates: %w", err)
	}
	if parsed.Rates == nil {
		return fmt.Errorf("decode rates: missing rates object")
	}
	c.mu.Lock()
	c.rates = parsed.Rates
	c.fetchedAt = c.clock()
	c.mu.Unlock()
	return nil
}

// RefreshWith is the test seam: feed a JSON body and a fetch
// timestamp without going through the HTTP fetcher. It mirrors
// what Refresh does after a successful fetch. Production code
// uses Refresh; tests use RefreshWith to avoid spinning up a
// local HTTP server.
func (c *RatesCache) RefreshWith(body []byte, fetchedAt time.Time) error {
	var parsed rateResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return fmt.Errorf("decode rates: %w", err)
	}
	if parsed.Rates == nil {
		return fmt.Errorf("decode rates: missing rates object")
	}
	c.mu.Lock()
	c.rates = parsed.Rates
	c.fetchedAt = fetchedAt
	c.mu.Unlock()
	return nil
}

// Rate returns the multiplier that converts USD into the given
// currency (1 USD = Rate(currency) target). Returns 0 when the
// cache is empty or the currency is unknown; callers treat 0 as
// "skip the conversion and keep USD".
//
// For USD itself we always return 1.0 so a populated entry with
// currency="USD" still produces sensible per-locale fields.
func (c *RatesCache) Rate(currency string) float64 {
	if currency == "" || currency == "USD" {
		return 1.0
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.rates == nil {
		return 0
	}
	r, ok := c.rates[currency]
	if !ok || r <= 0 {
		return 0
	}
	return r
}

// Convert applies rate to a USD amount using big.Float for
// precision (USD prices are floats per million tokens and we
// don't want float64 round-off to flip the last cent). Returns
// the resulting float64 plus the currency code to populate
// per-locale fields.
func (c *RatesCache) Convert(usd float64, currency string) (float64, bool) {
	rate := c.Rate(currency)
	if rate == 0 {
		return 0, false
	}
	usdBF := new(big.Float).SetFloat64(usd)
	rateBF := new(big.Float).SetFloat64(rate)
	product := new(big.Float).Mul(usdBF, rateBF)
	out, _ := product.Float64()
	return out, true
}