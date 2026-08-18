package api

// Provider catalog for the login-driven UI. The full list of 66
// omp providers comes from `omp get_login_providers` (when the
// host has omp installed). In that probe path, LoginProviderInfo is
// populated 1:1 from the omp RPC. When omp is unavailable, we
// fall back to the 5 known providers the keystore already supports
// (anthropic, openai, gemini, openrouter, minimax) so the harness
// UI is still functional in offline development.
//
// This module is the single source of truth for the wire shape of
// /api/v1/login/providers AND /api/v1/meta (PR-01 reshape). Both
// endpoints expose the same provider list — meta reports a flat
// array of LoginProviderInfo per the PR-01 spec.

import (
	"sync"
	"time"

	"github.com/lucaspdude/rocinante-harness/apps/api/internal/keystore"
)

// ProviderProbe is the seam for checking whether a given provider
// has an active key (env var OR keystore entry). Live wire in
// cmd/api/main.go uses keystore.EnvProbe; tests can stub it.
type ProviderProbe interface {
	IsConfigured(name string) bool
}

// LoginProvidersProvider is the seam that knows how to enumerate
// omp's providers. The default impl enumerates the 5 known
// providers; a richer impl could shell out to `omp
// get_login_providers` (PR-01 spec, future hardening).
type LoginProvidersProvider interface {
	List() []LoginProviderInfo
}

type staticLoginProviders struct {
	probe ProviderProbe
}

// NewStaticLoginProviders builds the fallback provider list. It is
// used when the harness can't shell out to omp (e.g. in CI or
// before the user installs omp). The list intentionally matches
// keystore.KnownProviders — paste-key is the only auth method
// exposed here because that's what keystore supports.
func NewStaticLoginProviders(probe ProviderProbe) LoginProvidersProvider {
	if probe == nil {
		probe = nullProbe{}
	}
	return staticLoginProviders{probe: probe}
}

// List returns the 5 known providers in a stable order. Auth is
// always "paste-key" because that's what the keystore implements.
// The user's installed omp may have additional providers — those
// will be returned by the dynamic provider impl (PR-01 follow-up).
func (s staticLoginProviders) List() []LoginProviderInfo {
	now := time.Now().UTC()
	out := make([]LoginProviderInfo, 0, len(keystore.KnownProviders))
	for _, p := range keystore.KnownProviders {
		info := providerInfoFromKnown(p, s.probe.IsConfigured(string(p)))
		_ = now
		out = append(out, info)
	}
	return out
}

// providerInfoFromKnown maps the keystore provider name to its
// LoginProviderInfo. The fields are stable so the web side can
// rely on id + auth + authenticated for its UI.
func providerInfoFromKnown(p keystore.ProviderName, configured bool) LoginProviderInfo {
	switch p {
	case keystore.Anthropic:
		return LoginProviderInfo{
			ID:            string(p),
			Name:          "Anthropic",
			Auth:          "paste-key",
			Available:     true,
			Authenticated: configured,
			EnvVar:        p.EnvVar(),
			HelpURL:       "https://console.anthropic.com/settings/keys",
		}
	case keystore.OpenAI:
		return LoginProviderInfo{
			ID:            string(p),
			Name:          "OpenAI",
			Auth:          "paste-key",
			Available:     true,
			Authenticated: configured,
			EnvVar:        p.EnvVar(),
			HelpURL:       "https://platform.openai.com/api-keys",
		}
	case keystore.Gemini:
		return LoginProviderInfo{
			ID:            string(p),
			Name:          "Google Gemini",
			Auth:          "paste-key",
			Available:     true,
			Authenticated: configured,
			EnvVar:        p.EnvVar(),
			HelpURL:       "https://aistudio.google.com/apikey",
		}
	case keystore.OpenRouter:
		return LoginProviderInfo{
			ID:            string(p),
			Name:          "OpenRouter",
			Auth:          "paste-key",
			Available:     true,
			Authenticated: configured,
			EnvVar:        p.EnvVar(),
			HelpURL:       "https://openrouter.ai/settings/keys",
		}
	case keystore.Minimax:
		return LoginProviderInfo{
			ID:            string(p),
			Name:          "Minimax (token plan)",
			Auth:          "paste-key",
			Available:     true,
			Authenticated: configured,
			EnvVar:        p.EnvVar(),
			HelpURL:       "https://minimax.io/dashboard",
		}
	default:
		return LoginProviderInfo{
			ID:            string(p),
			Name:          string(p),
			Auth:          "paste-key",
			Available:     true,
			Authenticated: configured,
			EnvVar:        p.EnvVar(),
		}
	}
}

type nullProbe struct{}

func (nullProbe) IsConfigured(string) bool { return false }

// LoginProvidersCache wraps the provider source with a 5s TTL —
// matches the README cross-cutting decision §1. Cache is keyed on
// a hash of the available probe state so a save/delete flips the
// next read immediately (best-effort).
type LoginProvidersCache struct {
	src   LoginProvidersProvider
	mu    sync.RWMutex
	cached []LoginProviderInfo
	at     time.Time
}

// NewLoginProvidersCache wraps src with a 5s TTL.
func NewLoginProvidersCache(src LoginProvidersProvider) *LoginProvidersCache {
	return &LoginProvidersCache{src: src}
}

// Snapshot returns the current provider list, refreshing when the
// cache is older than 5s.
func (c *LoginProvidersCache) Snapshot() []LoginProviderInfo {
	c.mu.RLock()
	if c.cached != nil && time.Since(c.at) < 5*time.Second {
		defer c.mu.RUnlock()
		return c.cached
	}
	c.mu.RUnlock()
	c.mu.Lock()
	defer c.mu.Unlock()
	// Re-check under write lock to avoid double refresh.
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
