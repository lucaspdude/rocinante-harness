package omp

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
)

// MetaResponse is the JSON body of /api/v1/meta.
//
// Providers is a flat array of provider info entries with the
// same wire shape as catalog.LoginProviderInfo (capabilities
// rather than a single auth string).
//
// DefaultModel is the server's pre-selected model id (PR-02).
// Resolved from OMP_DEFAULT_MODEL at request time; falls back
// to the first selectable model from the catalog handler when
// the env var is empty. Empty string = no default (client
// should fall back to its own saved selection or leave empty).
type MetaResponse struct {
	APIVersion      string             `json:"api_version"`
	OmpVersion      string             `json:"omp_version"`
	ProtocolVersion int                `json:"protocol_version"`
	OmpBin          string             `json:"omp_bin"`
	DefaultModel    string             `json:"default_model,omitempty"`
	Providers       []MetaProviderInfo `json:"providers"`
}

// MetaProviderInfo is the per-provider entry in MetaResponse. Wire
// shape matches catalog.LoginProviderInfo after the PR-01 review
// follow-up: capabilities (EnvVars + SupportsLogin + Keyless)
// instead of a single auth string.
type MetaProviderInfo struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Available     bool     `json:"available"`
	Authenticated bool     `json:"authenticated"`
	EnvVars       []string `json:"env_vars,omitempty"`
	SupportsLogin bool     `json:"supports_login"`
	Keyless       bool     `json:"keyless,omitempty"`
	HelpURL       string   `json:"help_url,omitempty"`
}

// ProviderProbe is the dependency the meta handler needs.
type ProviderProbe interface {
	IsConfigured(name string) bool
}

// Phase 7 — item 02: respondMeta is the common helper used by
// BOTH the GET /api/v1/meta and the POST /api/v1/meta/refresh
// handlers. Wire shape is identical; status codes match the
// loader state.
//
// Branches:
//   - bin == ""              → 503 omp_not_found
//   - bin != "", version ""  → 503 omp_version_unknown (NEW; the
//                              pre-phase-7 code returned 200 with
//                              an empty version, which is the bug)
//   - bin != "", version OK  → 200 with full MetaResponse
func respondMeta(
	w http.ResponseWriter,
	r *http.Request,
	loader Loader,
	apiVersion string,
	providers []MetaProviderInfo,
	defaultModel string,
) {
	bin := loader.OmpBin()
	if bin == "" {
		writeMetaError(w, r, http.StatusServiceUnavailable, "omp_not_found",
			"omp binary not found on PATH or $OMP_BIN")
		return
	}
	protocol, version := loader.OmpVersion()
	if protocol == 0 && version == "" {
		// 503 omp_version_unknown — the loader has a bin path
		// but no probed metadata yet. Logged so operators can
		// see WHY the meta is empty.
		if dl, ok := loader.(*DynamicLoader); ok {
			log.Printf("api: serving /meta with omp_version_unknown "+
				"(bin=%q, last_err=%q)", bin, dl.LastError())
		}
		writeMetaError(w, r, http.StatusServiceUnavailable, "omp_version_unknown",
			"omp binary found but version probe failed; see api logs")
		return
	}
	if providers == nil {
		providers = []MetaProviderInfo{}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(MetaResponse{
		APIVersion:      apiVersion,
		OmpVersion:      version,
		ProtocolVersion: protocol,
		OmpBin:          bin,
		DefaultModel:    defaultModel,
		Providers:       providers,
	})
}

// writeMetaError serializes a JSON error body with the same
// { code, message } shape used by the rest of the api.
func writeMetaError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	w.Header().Set("Access-Control-Allow-Origin", r.Header.Get("Origin"))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"code":    code,
		"message": message,
	})
}

// NewMetaHandler returns the http.HandlerFunc for /api/v1/meta.
// Delegates to respondMeta.
func NewMetaHandler(loader Loader, apiVersion string, providers []MetaProviderInfo, defaultModel string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		respondMeta(w, r, loader, apiVersion, providers, defaultModel)
	}
}

// NewMetaRefreshHandler returns the http.HandlerFunc for
// POST /api/v1/meta/refresh. Triggers a synchronous omp
// re-probe (bounded by the loader's probeBudget) and then
// serves the same MetaResponse as the GET handler. Useful
// for the web's "Re-check now" button after a transient
// failure.
//
// errProbeInFlight is treated as a no-op success (200 with
// the current snapshot). All other errors are logged at warn
// level but do not block the 200 response — the caller can
// still see the current state.
func NewMetaRefreshHandler(loader *DynamicLoader, apiVersion string, providers []MetaProviderInfo, defaultModel string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		probeBudget := defaultProbeBudget
		// Best-effort: respect the configured budget.
		if loader != nil {
			probeBudget = loader.probeBudget
		}
		ctx, cancel := context.WithTimeout(r.Context(), probeBudget)
		defer cancel()
		if loader != nil {
			if err := loader.Recheck(ctx); err != nil && !errors.Is(err, ErrProbeInFlight) {
				log.Printf("api: omp refresh probe failed: %v", err)
			}
		}
		respondMeta(w, r, loader, apiVersion, providers, defaultModel)
	}
}

