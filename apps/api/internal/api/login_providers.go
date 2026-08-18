package api

// Provider catalog for the login-driven UI. PR-01 (static fallback)
// + post-review F1 (dynamic ompRPC impl).
//
// The merge of static + dynamic is delegated to `Merge` which
// returns a merged slice with the dynamic list taking precedence
// per id (so an omp upgrade that adds providers shows them).

import (
	"sort"
	"sync"
	"time"

	"github.com/lucaspdude/rocinante-harness/apps/api/internal/catalog"
	"github.com/lucaspdude/rocinante-harness/apps/api/internal/keystore"
)

// ProviderProbe is the seam for checking whether a given provider
// has an active key (env var OR keystore entry). Live wire in
// cmd/api/main.go uses keystore.EnvProbe; tests can stub it.
type ProviderProbe interface {
	IsConfigured(name string) bool
}

// LoginProvidersProvider is the seam that knows how to enumerate
// omp's providers.
type LoginProvidersProvider interface {
	List() []catalog.LoginProviderInfo
}

// NewStaticLoginProviders returns the fallback list (5 known
// paste-key providers). Used when omp is unavailable; the runtime
// wiring in main prefers NewDynamicLoginProviders which falls
// back to the static impl if omp spawn fails.
func NewStaticLoginProviders(probe ProviderProbe) LoginProvidersProvider {
	if probe == nil {
		probe = nullProbe{}
	}
	return staticLoginProviders{probe: probe}
}

type staticLoginProviders struct {
	probe ProviderProbe
}

// List returns the 5 known providers in stable order with their
// single canonical env var. Auth method is paste-key because the
// keystore only handles that today. The list is sorted by id so
// callers don't have to.
func (s staticLoginProviders) List() []catalog.LoginProviderInfo {
	out := []catalog.LoginProviderInfo{
		providerInfoFromKnown(keystore.Anthropic, s.probe.IsConfigured(string(keystore.Anthropic))),
		providerInfoFromKnown(keystore.OpenAI, s.probe.IsConfigured(string(keystore.OpenAI))),
		providerInfoFromKnown(keystore.Gemini, s.probe.IsConfigured(string(keystore.Gemini))),
		providerInfoFromKnown(keystore.OpenRouter, s.probe.IsConfigured(string(keystore.OpenRouter))),
		providerInfoFromKnown(keystore.Minimax, s.probe.IsConfigured(string(keystore.Minimax))),
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// providerInfoFromKnown maps the keystore provider name to its
// LoginProviderInfo. Every known paste-key provider supports the
// generic `/login` OAuth-style flow. Keyless is false (each
// requires an API key).
func providerInfoFromKnown(p keystore.ProviderName, configured bool) catalog.LoginProviderInfo {
	var help, name string
	switch p {
	case keystore.Anthropic:
		help = "https://console.anthropic.com/settings/keys"
		name = "Anthropic"
	case keystore.OpenAI:
		help = "https://platform.openai.com/api-keys"
		name = "OpenAI"
	case keystore.Gemini:
		help = "https://aistudio.google.com/apikey"
		name = "Google Gemini"
	case keystore.OpenRouter:
		help = "https://openrouter.ai/settings/keys"
		name = "OpenRouter"
	case keystore.Minimax:
		help = "https://minimax.io/dashboard"
		name = "Minimax (token plan)"
	}
	if name == "" {
		name = string(p)
	}
	return catalog.LoginProviderInfo{
		ID:            string(p),
		Name:          name,
		Available:     true,
		Authenticated: configured,
		EnvVars:       []string{p.EnvVar()},
		SupportsLogin: true,
		Keyless:       false,
		HelpURL:       help,
	}
}

type nullProbe struct{}

func (nullProbe) IsConfigured(string) bool { return false }

// LoginProvidersCache wraps the provider source with a 5s TTL —
// matches the README cross-cutting decision §1.
type LoginProvidersCache struct {
	src    LoginProvidersProvider
	mu     sync.RWMutex
	cached []catalog.LoginProviderInfo
	at     time.Time
}

// NewLoginProvidersCache wraps src with a 5s TTL.
func NewLoginProvidersCache(src LoginProvidersProvider) *LoginProvidersCache {
	return &LoginProvidersCache{src: src}
}

// Snapshot returns the current provider list, refreshing when the
// cache is older than 5s.
func (c *LoginProvidersCache) Snapshot() []catalog.LoginProviderInfo {
	c.mu.RLock()
	if c.cached != nil && time.Since(c.at) < 5*time.Second {
		defer c.mu.RUnlock()
		return c.cached
	}
	c.mu.RUnlock()
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cached != nil && time.Since(c.at) < 5*time.Second {
		return c.cached
	}
	c.cached = c.src.List()
	c.at = time.Now().UTC()
	return c.cached
}

// Invalidate forces the next Snapshot call to refresh.
func (c *LoginProvidersCache) Invalidate() {
	c.mu.Lock()
	c.cached = nil
	c.at = time.Time{}
	c.mu.Unlock()
}
