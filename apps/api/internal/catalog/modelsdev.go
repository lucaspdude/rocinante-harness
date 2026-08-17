// Package catalog fetches models.dev's public catalog of LLM models
// and serves a flat list with cross-references to omp's runtime
// provider list. See PR-02 (docs/mvp/phase-1-functionality/05-pr-specs/
// PR-02-models-catalog.md) for the wire format.
package catalog

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

// ModelsDevEntry is a single model entry returned by the catalog
// endpoint. Models.dev nests the data as {provider: {models: {id: ...}}};
// we flatten to a single array.
type ModelsDevEntry struct {
	ID            string   `json:"id"`
	Provider      string   `json:"provider"`
	Name          string   `json:"name"`
	ContextLength int      `json:"context_length,omitempty"`
	Modalities    []string `json:"modalities,omitempty"`
	CostInput     float64  `json:"cost_input,omitempty"`
	CostOutput    float64  `json:"cost_output,omitempty"`
	Selectable    bool     `json:"selectable"`
	Stale         bool     `json:"stale,omitempty"`
}

// ModelsDevCatalog owns the in-memory cache of models.dev entries
// and the cross-reference to omp's login providers + available
// models.
type ModelsDevCatalog struct {
	mu        sync.RWMutex
	entries   []ModelsDevEntry
	expiresAt time.Time
	inFlight  chan struct{}
	ttl       time.Duration
	hc        *http.Client
	fetcher   Fetcher
}

// Fetcher returns the raw models.dev JSON bytes.
type Fetcher func(ctx context.Context) ([]byte, error)

// NewModelsDevCatalog returns an empty catalog with the default 1h
// TTL.
func NewModelsDevCatalog() *ModelsDevCatalog {
	return &ModelsDevCatalog{
		ttl:      time.Hour,
		inFlight: make(chan struct{}, 1),
		hc: &http.Client{
			Timeout: 15 * time.Second,
		},
		fetcher: defaultFetcher(),
	}
}

// SetFetcher swaps the underlying fetcher (test seam).
func (c *ModelsDevCatalog) SetFetcher(f Fetcher) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.fetcher = f
}

func defaultFetcher() Fetcher {
	return func(ctx context.Context) ([]byte, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://models.dev/api.json", nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "rocinante-harness/0.1.0")
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, err
		}
		defer res.Body.Close()
		if res.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("models.dev status %d", res.StatusCode)
		}
		body, err := io.ReadAll(res.Body)
		if err != nil {
			return nil, err
		}
		return body, nil
	}
}

// Refresh forces a cache refresh.
func (c *ModelsDevCatalog) Refresh(ctx context.Context) error {
	c.mu.RLock()
	inFlight := c.inFlight
	c.mu.RUnlock()
	select {
	case inFlight <- struct{}{}:
		// claimed
	default:
		select {
		case <-inFlight:
			c.mu.RLock()
			fresh := !c.expiresAt.IsZero() && time.Now().Before(c.expiresAt)
			c.mu.RUnlock()
			if fresh {
				return nil
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	defer func() {
		c.mu.Lock()
		<-inFlight
		c.mu.Unlock()
	}()

	c.mu.RLock()
	fetcher := c.fetcher
	c.mu.RUnlock()
	body, err := fetcher(ctx)
	if err != nil {
		c.mu.Lock()
		c.staleLocked()
		c.mu.Unlock()
		return err
	}
	entries, err := flattenModelsDev(body)
	if err != nil {
		c.mu.Lock()
		c.staleLocked()
		c.mu.Unlock()
		return err
	}
	c.mu.Lock()
	c.entries = entries
	c.expiresAt = time.Now().Add(c.ttl)
	c.mu.Unlock()
	return nil
}

func (c *ModelsDevCatalog) staleLocked() {
	if c.entries == nil {
		c.entries = []ModelsDevEntry{}
	}
	for i := range c.entries {
		c.entries[i].Stale = true
	}
}

// Snapshot returns the current catalog entries (best-effort).
func (c *ModelsDevCatalog) Snapshot() []ModelsDevEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.entries == nil {
		return []ModelsDevEntry{}
	}
	out := make([]ModelsDevEntry, len(c.entries))
	copy(out, c.entries)
	return out
}

// Stale returns true if the last refresh attempt failed and
// callers should treat the catalog as cached-but-stale.
func (c *ModelsDevCatalog) Stale() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.entries == nil {
		return true
	}
	for _, e := range c.entries {
		if e.Stale {
			return true
		}
	}
	return false
}

// flattenModelsDev parses the raw models.dev JSON into a flat list
// sorted by provider + id.
func flattenModelsDev(body []byte) ([]ModelsDevEntry, error) {
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	out := make([]ModelsDevEntry, 0, 256)
	for provider, payload := range raw {
		payloadMap, _ := payload.(map[string]any)
		if payloadMap == nil {
			continue
		}
		modelsAny, _ := payloadMap["models"].(map[string]any)
		if modelsAny == nil {
			continue
		}
		for id, fields := range modelsAny {
			fmap, _ := fields.(map[string]any)
			entry := ModelsDevEntry{
				ID:       id,
				Provider: provider,
				Name:     id,
			}
			if v, ok := fmap["name"].(string); ok {
				entry.Name = v
			}
			if v, ok := fmap["context_length"].(float64); ok {
				entry.ContextLength = int(v)
			}
			if v, ok := fmap["cost_input"].(float64); ok {
				entry.CostInput = v
			}
			if v, ok := fmap["cost_output"].(float64); ok {
				entry.CostOutput = v
			}
			if mods, ok := fmap["modalities"].(map[string]any); ok {
				if m, ok := mods["input"].([]any); ok {
					for _, x := range m {
						if s, ok := x.(string); ok {
							entry.Modalities = append(entry.Modalities, "input:"+s)
						}
					}
				}
				if m, ok := mods["output"].([]any); ok {
					for _, x := range m {
						if s, ok := x.(string); ok {
							entry.Modalities = append(entry.Modalities, "output:"+s)
						}
					}
				}
			}
			out = append(out, entry)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Provider != out[j].Provider {
			return out[i].Provider < out[j].Provider
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

// Search applies the canonical search ranking.
func Search(entries []ModelsDevEntry, query, providerFilter string, selectableOnly bool, limit int) []ModelsDevEntry {
	q := strings.ToLower(strings.TrimSpace(query))
	out := make([]ModelsDevEntry, 0, len(entries))
	for _, e := range entries {
		if selectableOnly && !e.Selectable {
			continue
		}
		if providerFilter != "" && !strings.EqualFold(e.Provider, providerFilter) {
			continue
		}
		idLower := strings.ToLower(e.ID)
		nameLower := strings.ToLower(e.Name)
		if q == "" {
			out = append(out, e)
			continue
		}
		if idLower == q || nameLower == q {
			out = append(out, e)
			continue
		}
		if strings.HasPrefix(idLower, q) || strings.HasPrefix(nameLower, q) {
			out = append(out, e)
			continue
		}
		if strings.Contains(idLower, q) || strings.Contains(nameLower, q) {
			out = append(out, e)
			continue
		}
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}
