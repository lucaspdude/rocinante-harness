package catalog

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestCurrencyForLocale(t *testing.T) {
	cases := []struct {
		locale string
		want   string
	}{
		{"en-US", "USD"},
		{"pt-BR", "BRL"},
		{"en-GB", "GBP"},
		{"de-DE", "EUR"},
		{"ja-JP", "JPY"},
		{"zh-CN", "CNY"},
		{"fr-FR", "USD"}, // unmapped default
		{"", "USD"},
		{"xx-XX", "USD"},
	}
	for _, c := range cases {
		got := CurrencyForLocale(c.locale)
		if got != c.want {
			t.Errorf("CurrencyForLocale(%q) = %q, want %q", c.locale, got, c.want)
		}
	}
}

func TestParseRates(t *testing.T) {
	body := []byte(`{
		"base": "USD",
		"date": "2026-08-19",
		"rates": {"USD": 1.0, "BRL": 5.10, "EUR": 0.85, "GBP": 0.75, "JPY": 152.0, "CNY": 7.10}
	}`)
	c := NewRatesCache()
	if err := c.RefreshWith(body, time.Now()); err != nil {
		t.Fatalf("seed: %v", err)
	}
	want := 5.10
	if got := c.Rate("BRL"); got != want {
		t.Errorf("Rate(BRL) = %v, want %v", got, want)
	}
	if got := c.Rate("USD"); got != 1.0 {
		t.Errorf("Rate(USD) = %v, want 1.0", got)
	}
	if got := c.Rate("ZZZ"); got != 0 {
		t.Errorf("Rate(ZZZ) = %v, want 0", got)
	}
}

func TestConvertUsesBigFloat(t *testing.T) {
	c := NewRatesCache()
	if err := c.RefreshWith([]byte(`{"rates":{"BRL":5.123456789,"USD":1.0}}`), time.Now()); err != nil {
		t.Fatalf("seed: %v", err)
	}
	got, ok := c.Convert(3.0, "BRL")
	if !ok {
		t.Fatal("Convert should succeed for BRL")
	}
	want := 3.0 * 5.123456789
	if got < want-1e-9 || got > want+1e-9 {
		t.Errorf("Convert(3 USD → BRL) = %v, want ~%v", got, want)
	}
}

func TestConvertSkipsUnknownCurrency(t *testing.T) {
	c := NewRatesCache()
	if err := c.RefreshWith([]byte(`{"rates":{"USD":1.0}}`), time.Now()); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if _, ok := c.Convert(3.0, "XYZ"); ok {
		t.Error("Convert should fail for unknown currency")
	}
}

func TestRatesCacheTTLExpiry(t *testing.T) {
	c := NewRatesCache()
	base := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)
	clk := base
	c.SetClock(func() time.Time { return clk })
	calls := 0
	c.SetFetcher(func(ctx context.Context) ([]byte, error) {
		calls++
		return []byte(`{"rates":{"BRL":5.0,"USD":1.0}}`), nil
	})

	if err := c.Refresh(context.Background()); err != nil {
		t.Fatalf("first refresh: %v", err)
	}
	if calls != 1 {
		t.Fatalf("calls after first refresh = %d, want 1", calls)
	}

	// 1 hour later — cache still fresh.
	clk = base.Add(time.Hour)
	if err := c.Refresh(context.Background()); err != nil {
		t.Fatalf("warm refresh: %v", err)
	}
	if calls != 1 {
		t.Errorf("calls after warm refresh = %d, want 1 (cache hit)", calls)
	}

	// 23h59m later — still fresh.
	clk = base.Add(23*time.Hour + 59*time.Minute)
	if err := c.Refresh(context.Background()); err != nil {
		t.Fatalf("almost-stale refresh: %v", err)
	}
	if calls != 1 {
		t.Errorf("calls after almost-stale = %d, want 1 (cache hit)", calls)
	}

	// 24h+1s later — cache expired, must re-fetch.
	clk = base.Add(24*time.Hour + time.Second)
	if err := c.Refresh(context.Background()); err != nil {
		t.Fatalf("expired refresh: %v", err)
	}
	if calls != 2 {
		t.Errorf("calls after expired = %d, want 2", calls)
	}
}

func TestRatesCacheKeepsStaleOnFetchError(t *testing.T) {
	c := NewRatesCache()
	base := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)
	c.SetClock(func() time.Time { return base })

	// Seed with a known rate via the test seam so we don't have
	// to mock the HTTP fetcher twice (success path + failure path).
	if err := c.RefreshWith([]byte(`{"rates":{"BRL":5.0,"USD":1.0}}`), base); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if c.Rate("BRL") == 0 {
		t.Fatalf("expected BRL rate > 0 after seed")
	}
	originalRate := c.Rate("BRL")

	// Replace with a fetcher that always fails; the cache should
	// keep the prior rate so callers still see something useful.
	c.SetFetcher(func(ctx context.Context) ([]byte, error) {
		return nil, &testErr{"upstream down"}
	})
	c.SetClock(func() time.Time { return base.Add(48 * time.Hour) })
	if err := c.Refresh(context.Background()); err == nil {
		t.Fatal("Refresh should return error on fetch failure")
	}
	if c.Rate("BRL") != originalRate {
		t.Errorf("BRL rate changed after fetch failure: got %v, want %v", c.Rate("BRL"), originalRate)
	}
}

func TestRatesCacheEmptyMapDoesNotConvert(t *testing.T) {
	c := NewRatesCache()
	// No Refresh call: rates map is nil.
	if _, ok := c.Convert(3.0, "BRL"); ok {
		t.Error("Convert on empty cache should not succeed")
	}
	if c.Rate("USD") != 1.0 {
		t.Errorf("Rate(USD) without cache = %v, want 1.0", c.Rate("USD"))
	}
}

func TestRatesCacheRejectsMalformedJSON(t *testing.T) {
	c := NewRatesCache()
	err := c.RefreshWith([]byte(`{not json`), time.Now())
	if err == nil {
		t.Fatal("expected error on malformed JSON")
	}
	if !strings.Contains(err.Error(), "decode") {
		t.Errorf("error %q should mention decode", err.Error())
	}
}

func TestRatesCacheRejectsMissingRatesObject(t *testing.T) {
	c := NewRatesCache()
	err := c.RefreshWith([]byte(`{"base":"USD"}`), time.Now())
	if err == nil {
		t.Fatal("expected error when rates object missing")
	}
}

// testErr is a tiny helper used to assert the error path is
// reachable without pulling in fmt for every test.
type testErr struct{ msg string }

func (e *testErr) Error() string { return e.msg }