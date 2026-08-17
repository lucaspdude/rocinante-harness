package catalog

import (
	"context"
	"testing"
)

func TestFlattenModelsDev(t *testing.T) {
	body := []byte(`{
		"openai": {
			"models": {
				"gpt-5": {"name": "GPT 5", "context_length": 400000, "modalities": {"input": ["text"], "output": ["text"]}},
				"o3-mini": {"name": "o3 mini"}
			}
		},
		"anthropic": {
			"models": {
				"claude-opus": {"name": "Claude Opus", "context_length": 200000, "cost_input": 15.0, "cost_output": 75.0}
			}
		}
	}`)
	entries, err := flattenModelsDev(body)
	if err != nil {
		t.Fatalf("flatten: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("len = %d, want 3", len(entries))
	}
	if entries[0].Provider != "anthropic" {
		t.Errorf("entries[0].Provider = %q, want anthropic", entries[0].Provider)
	}
	if entries[1].ID != "gpt-5" {
		t.Errorf("entries[1].ID = %q, want gpt-5", entries[1].ID)
	}
	if entries[0].ContextLength != 200000 {
		t.Errorf("entries[0].ContextLength = %d", entries[0].ContextLength)
	}
	if entries[1].Name != "GPT 5" {
		t.Errorf("entries[1].Name = %q, want GPT 5", entries[1].Name)
	}
}

func TestSearchRanking(t *testing.T) {
	entries := []ModelsDevEntry{
		{ID: "claude-opus-4", Provider: "anthropic", Name: "Claude Opus 4"},
		{ID: "claude-sonnet", Provider: "anthropic", Name: "Claude Sonnet"},
		{ID: "gpt-5", Provider: "openai", Name: "GPT 5"},
	}

	got := Search(entries, "claude-opus-4", "", false, 0)
	if len(got) != 1 || got[0].ID != "claude-opus-4" {
		t.Errorf("exact: got %+v", got)
	}
	got = Search(entries, "claude", "", false, 0)
	if len(got) != 2 {
		t.Errorf("prefix: got %d, want 2", len(got))
	}
	got = Search(entries, "", "openai", false, 0)
	if len(got) != 1 || got[0].ID != "gpt-5" {
		t.Errorf("provider: got %+v", got)
	}
	entries[2].Selectable = true
	got = Search(entries, "", "", true, 0)
	if len(got) != 1 || got[0].ID != "gpt-5" {
		t.Errorf("selectable: got %+v", got)
	}
}

func TestRefreshWithFetcher(t *testing.T) {
	c := NewModelsDevCatalog()
	c.SetFetcher(func(_ context.Context) ([]byte, error) {
		return []byte(`{"x":{"models":{"y":{"name":"Y"}}}}`), nil
	})
	if err := c.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	entries := c.Snapshot()
	if len(entries) != 1 || entries[0].ID != "y" {
		t.Fatalf("snapshot = %+v", entries)
	}
}

func TestStaleOnError(t *testing.T) {
	c := NewModelsDevCatalog()
	c.SetFetcher(func(_ context.Context) ([]byte, error) {
		return []byte(`{"x":{"models":{"y":{}}}}`), nil
	})
	_ = c.Refresh(context.Background())
	c.SetFetcher(func(_ context.Context) ([]byte, error) {
		return nil, context.DeadlineExceeded
	})
	_ = c.Refresh(context.Background())
	if !c.Stale() {
		t.Error("expected stale after failed refresh")
	}
}

func TestAnnotateSelectable(t *testing.T) {
	entries := []ModelsDevEntry{
		{ID: "gpt-5", Provider: "openai"},
		{ID: "claude-opus", Provider: "anthropic"},
		{ID: "unknown-model", Provider: "nonexistent"},
	}
	providers := []LoginProviderInfo{
		{ID: "openai", Available: true, Authenticated: true},
		{ID: "anthropic", Available: true, Authenticated: false},
	}
	AnnotateSelectable(entries, providers)
	if !entries[0].Selectable {
		t.Error("gpt-5 should be selectable")
	}
	if entries[1].Selectable {
		t.Error("claude-opus should NOT be selectable (not authenticated)")
	}
	if entries[2].Selectable {
		t.Error("unknown-model should NOT be selectable")
	}
}
