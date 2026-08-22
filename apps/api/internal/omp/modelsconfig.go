// Package omp — models.yml writer (phase-6 PR-1).
//
// The api is the source of truth for which providers the OMP
// should be aware of. When a provider key is added (POST
// /api/v1/providers/{name}/key) or removed (DELETE), the keystore
// updates provider-keys.json and we *also* rewrite the OMP's
// models.yml so the OMP subprocess sees the updated provider list on
// its next spawn.
//
// File format (matches the OMP probe output schema):
//
//	providers:
//	  - id: anthropic
//	    api: anthropic-messages
//	    apiKey: ${ANTHROPIC_API_KEY}      # expanded at spawn time
//	    baseUrl: https://api.anthropic.com
//	    models:
//	      - id: claude-sonnet-4
//	  - id: openai
//	    api: openai-chat
//	    apiKey: ${OPENAI_API_KEY}
//	    baseUrl: https://api.openai.com/v1
//	    models:
//	      - id: gpt-4o
//	      - id: gpt-4o-mini
//
// We keep the file at $OMP_AGENT_DIR/models.yml (default
// $HOME/.omp/agent/models.yml — the same dir the OMP probe uses).
// The writer is atomic (temp file + rename) and idempotent:
// re-running with the same keystore state produces the same file.
package omp

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/lucaspdude/rocinante-harness/apps/api/internal/keystore"
)

// ModelEntry is one model under a provider.
type ModelEntry struct {
	ID string
}

// ProviderEntry is one provider block in models.yml.
type ProviderEntry struct {
	ID      string
	API     string // "anthropic-messages" | "openai-completions" | etc.
	APIKey  string // env var name, e.g. ANTHROPIC_API_KEY
	BaseURL string
	Models  []ModelEntry
}

// ModelsConfig is the rendered `models.yml` payload.
type ModelsConfig struct {
	Providers []ProviderEntry
}

// DefaultModelMap maps a provider name to a curated list of well-known
// model ids. Used when the keystore is configured but we don't know
// which specific model ids the provider exposes yet (e.g. the user
// didn't go through /login to probe). The OMP probe will overwrite
// these later; this is just so the OMP can resolve `minimax/MiniMax-M3`
// before the probe completes.
var DefaultModelMap = map[string][]ModelEntry{
	"anthropic":  {{ID: "claude-sonnet-4"}},
	"openai":     {{ID: "gpt-4o"}, {ID: "gpt-4o-mini"}},
	"gemini":     {{ID: "gemini-2.0-flash"}},
	"openrouter": {{ID: "openrouter/auto"}},
	"minimax":    {{ID: "minimax/MiniMax-M3"}, {ID: "minimax/MiniMax-M2"}},
}

// DefaultProviderMap maps a provider name to its protocol + base URL.
// Mirrors the OMP's own probe defaults so we don't surprise the OMP
// with a different endpoint than it would have probed on its own.
var DefaultProviderMap = map[string]ProviderEntry{
	"anthropic": {
		ID:      "anthropic",
		API:     "anthropic-messages",
		APIKey:  "ANTHROPIC_API_KEY",
		BaseURL: "https://api.anthropic.com",
		Models:  DefaultModelMap["anthropic"],
	},
	"openai": {
		ID:      "openai",
		API:     "openai-completions",
		APIKey:  "OPENAI_API_KEY",
		BaseURL: "https://api.openai.com/v1",
		Models:  DefaultModelMap["openai"],
	},
	"gemini": {
		ID:      "gemini",
		API:     "google-generative-ai",
		APIKey:  "GEMINI_API_KEY",
		BaseURL: "https://generativelanguage.googleapis.com",
		Models:  DefaultModelMap["gemini"],
	},
	"openrouter": {
		ID:      "openrouter",
		API:     "openai-completions",
		APIKey:  "OPENROUTER_API_KEY",
		BaseURL: "https://openrouter.ai/api/v1",
		Models:  DefaultModelMap["openrouter"],
	},
	"minimax": {
		ID:      "minimax",
		API:     "openai-completions",
		APIKey:  "MINIMAX_API_KEY",
		BaseURL: "https://api.minimaxi.chat/v1",
		Models:  DefaultModelMap["minimax"],
	},
}

// ModelsConfigWriter writes models.yml from the keystore state.
type ModelsConfigWriter struct {
	dir  string // directory containing models.yml (default ~/.omp/agent/)
	mu   sync.Mutex
}

// NewModelsConfigWriter returns a writer rooted at dir. If dir is
// empty, the writer uses the OMP_AGENT_DIR env var, falling back to
// $HOME/.omp/agent/ (the same path the OMP probe uses).
func NewModelsConfigWriter(dir string) *ModelsConfigWriter {
	if dir == "" {
		dir = os.Getenv("OMP_AGENT_DIR")
	}
	if dir == "" {
		dir = filepath.Join(os.Getenv("HOME"), ".omp", "agent")
	}
	return &ModelsConfigWriter{dir: dir}
}

// Path returns the absolute path of the models.yml this writer manages.
func (w *ModelsConfigWriter) Path() string {
	return filepath.Join(w.dir, "models.yml")
}

// Render converts the keystore into a ModelsConfig. The
// configured providers (those with non-empty keys in the keystore)
// appear in the output; unconfigured ones are dropped so the OMP
// doesn't try to probe them on every spawn.
func Render(ks *keystore.Store) (ModelsConfig, error) {
	keys, err := ks.Snapshot()
	if err != nil {
		return ModelsConfig{}, fmt.Errorf("modelsconfig: read keystore: %w", err)
	}
	var out ModelsConfig
	for _, name := range keystore.KnownProviders {
		// Skip unconfigured providers.
		if _, ok := keys[string(name)]; !ok {
			continue
		}
		entry, ok := DefaultProviderMap[string(name)]
		if !ok {
			// Provider the keystore knows about but we don't
			// have a default entry for. Skip; the OMP probe
			// will pick it up next time the user clicks
			// "Discover".
			continue
		}
		out.Providers = append(out.Providers, entry)
	}
	return out, nil
}

// Sync writes models.yml from the current keystore state. Idempotent:
// re-running with the same state produces the same file. Atomic:
// writes to a temp file in the same dir and renames. Creates the
// parent dir if missing (chmod 0700).
func (w *ModelsConfigWriter) Sync(ks *keystore.Store) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	cfg, err := Render(ks)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(w.dir, 0o700); err != nil {
		return fmt.Errorf("modelsconfig: mkdir: %w", err)
	}
	content := marshalYAML(cfg)
	tmp, err := os.CreateTemp(w.dir, ".models.yml.*.tmp")
	if err != nil {
		return fmt.Errorf("modelsconfig: create temp: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write([]byte(content)); err != nil {
		tmp.Close()
		return fmt.Errorf("modelsconfig: write temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("modelsconfig: close temp: %w", err)
	}
	if err := os.Rename(tmpName, w.Path()); err != nil {
		return fmt.Errorf("modelsconfig: rename: %w", err)
	}
	return nil
}

// marshalYAML hand-prints the file rather than depending on a YAML
// library — the format is small, stable, and we want zero new deps.
// The output is what `omp --probe` would have written.
func marshalYAML(cfg ModelsConfig) string {
	var sb strings.Builder
	sb.WriteString("# Generated by the roc-harness api (phase-6 PR-1).\n")
	sb.WriteString("# Do not edit by hand — the api rewrites this file every\n")
	sb.WriteString("# time a provider key is added or removed via /settings.\n")
	sb.WriteString("providers:\n")
	for _, p := range cfg.Providers {
		fmt.Fprintf(&sb, "  - id: %s\n", p.ID)
		fmt.Fprintf(&sb, "    api: %s\n", p.API)
		fmt.Fprintf(&sb, "    apiKey: ${%s}\n", p.APIKey)
		fmt.Fprintf(&sb, "    baseUrl: %s\n", p.BaseURL)
		if len(p.Models) == 0 {
			sb.WriteString("    models: []\n")
			continue
		}
		sb.WriteString("    models:\n")
		for _, m := range p.Models {
			fmt.Fprintf(&sb, "      - id: %s\n", m.ID)
		}
	}
	return sb.String()
}

// ErrNotConfigured is returned by Sync when the keystore is empty.
var ErrNotConfigured = errors.New("modelsconfig: keystore has no providers; skipping models.yml write")

// SyncIfConfigured is Sync, but if the keystore has zero providers
// the file is left untouched (we don't want to wipe models.yml when
// the user has no providers configured — the OMP still has its
// probe-cached versions from install time).
func (w *ModelsConfigWriter) SyncIfConfigured(ks *keystore.Store) error {
	keys, err := ks.Snapshot()
	if err != nil {
		return fmt.Errorf("modelsconfig: read keystore: %w", err)
	}
	if len(keys) == 0 {
		return ErrNotConfigured
	}
	return w.Sync(ks)
}
