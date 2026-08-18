package omp

import (
	"encoding/json"
	"net/http"
)

// MetaResponse is the JSON body of /api/v1/meta.
//
// Providers is a flat array of provider info entries with the
// same wire shape as catalog.LoginProviderInfo (capabilities
// rather than a single auth string).
type MetaResponse struct {
	APIVersion      string             `json:"api_version"`
	OmpVersion      string             `json:"omp_version"`
	ProtocolVersion int                `json:"protocol_version"`
	OmpBin          string             `json:"omp_bin"`
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

// NewMetaHandler returns the http.HandlerFunc for /api/v1/meta.
// On success (omp_bin resolved), it returns 200. When the omp
// binary cannot be resolved, it returns 503 with a stable error
// code. providers is the source of provider rows.
func NewMetaHandler(loader Loader, apiVersion string, providers []MetaProviderInfo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		bin := loader.OmpBin()
		if bin == "" {
			w.Header().Set("Access-Control-Allow-Origin", r.Header.Get("Origin"))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"code":    "omp_not_found",
				"message": "omp binary not found on PATH or $OMP_BIN",
			})
			return
		}
		protocol, version := loader.OmpVersion()
		if providers == nil {
			providers = []MetaProviderInfo{}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(MetaResponse{
			APIVersion:      apiVersion,
			OmpVersion:      version,
			ProtocolVersion: protocol,
			OmpBin:          bin,
			Providers:       providers,
		})
	}
}
