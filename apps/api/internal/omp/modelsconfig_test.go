package omp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lucaspdude/rocinante-harness/apps/api/internal/keystore"
)

func TestModelsConfig_Render(t *testing.T) {
	tests := []struct {
		name     string
		keys     map[string]string
		wantSubs []string
		dontWant []string
	}{
		{
			name: "anthropic only",
			keys: map[string]string{
				"anthropic": "sk-test-anthropic",
			},
			wantSubs: []string{
				"api: anthropic-messages",
				"apiKey: ${ANTHROPIC_API_KEY}",
				"baseUrl: https://api.anthropic.com",
				"id: claude-sonnet-4",
			},
		},
		{
			name: "anthropic + minimax",
			keys: map[string]string{
				"anthropic": "sk-test-anthropic",
				"minimax":    "sk-test-minimax",
			},
			wantSubs: []string{
				"api: anthropic-messages",
				"api: openai-completions",
				"apiKey: ${MINIMAX_API_KEY}",
				"baseUrl: https://api.minimaxi.chat/v1",
				"id: claude-sonnet-4",
				"id: minimax/MiniMax-M3",
			},
		},
		{
			name:     "empty keystore produces empty config",
			keys:     map[string]string{},
			wantSubs: []string{"providers:\n"},
		},
		{
			name: "openrouter only",
			keys: map[string]string{
				"openrouter": "sk-or-test",
			},
			wantSubs: []string{
				"api: openai-completions",
				"apiKey: ${OPENROUTER_API_KEY}",
				"baseUrl: https://openrouter.ai/api/v1",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := renderFromMap(tt.keys)
			out := marshalYAML(cfg)
			for _, want := range tt.wantSubs {
				if !strings.Contains(out, want) {
					t.Errorf("output missing %q\ngot:\n%s", want, out)
				}
			}
			for _, dontWant := range tt.dontWant {
				if strings.Contains(out, dontWant) {
					t.Errorf("output contains %q but should not\ngot:\n%s", dontWant, out)
				}
			}
		})
	}
}

func TestModelsConfig_Sync_writesFile(t *testing.T) {
	dir := t.TempDir()
	w := NewModelsConfigWriter(filepath.Join(dir, "omp"))
	ks := keystore.New(dir)
	for k, v := range map[string]string{"anthropic": "sk-test-a", "minimax": "sk-test-m"} {
		if err := ks.Set(keystore.ProviderName(k), v); err != nil {
			t.Fatalf("set: %v", err)
		}
	}
	if err := w.Sync(ks); err != nil {
		t.Fatalf("sync: %v", err)
	}
	data, err := os.ReadFile(w.Path())
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if !strings.Contains(string(data), "id: minimax/MiniMax-M3") {
		t.Errorf("rendered file missing minimax/MiniMax-M3\ngot:\n%s", string(data))
	}
	if !strings.Contains(string(data), "apiKey: ${MINIMAX_API_KEY}") {
		t.Errorf("rendered file missing MINIMAX_API_KEY env var reference")
	}
}

func TestModelsConfig_PathOverride(t *testing.T) {
	dir := t.TempDir()
	custom := filepath.Join(dir, "sub", "models.yml")
	w := NewModelsConfigWriter(filepath.Dir(custom))
	if w.Path() != custom {
		t.Errorf("Path() = %q, want %q", w.Path(), custom)
	}
}

func TestModelsConfig_SyncIfConfigured_skipsWhenEmpty(t *testing.T) {
	dir := t.TempDir()
	w := NewModelsConfigWriter(filepath.Join(dir, "omp"))
	// Empty keystore: file should not be created.
	ks := keystore.New(dir)
	keys := map[string]string{}
	for k, v := range keys {
		if err := ks.Set(keystore.ProviderName(k), v); err != nil {
			t.Fatalf("set: %v", err)
		}
	}
	if err := w.SyncIfConfigured(ks); err == nil {
		t.Error("expected ErrNotConfigured for empty keystore, got nil")
	}
	if _, err := os.Stat(w.Path()); err == nil {
		t.Errorf("file should not have been created for empty keystore")
	}
}

// renderFromMap is a test helper that mirrors Render() but takes a
// plain map. Avoids the keystore.Store plumbing in tests where we
// just want to assert on the rendered YAML.
func renderFromMap(keys map[string]string) ModelsConfig {
	var out ModelsConfig
	for _, name := range keystore.KnownProviders {
		if _, ok := keys[string(name)]; !ok {
			continue
		}
		entry, ok := DefaultProviderMap[string(name)]
		if !ok {
			continue
		}
		out.Providers = append(out.Providers, entry)
	}
	return out
}
