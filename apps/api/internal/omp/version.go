package omp

import (
	"encoding/json"
	"net/http"
)

// MetaResponse is the JSON body of /api/v1/meta.
//
// Providers is a flat array of provider info entries rather than a
// map keyed by id. PR-01 reshaped this so the web side can render
// configured/unconfigured state for any provider omp discovers.
// The single-source-of-truth for the provider set lives in
// api.LoginProvidersCache — this response uses the same wire shape.
type MetaResponse struct {
	APIVersion      string             `json:"api_version"`
	OmpVersion      string             `json:"omp_version"`
	ProtocolVersion int                `json:"protocol_version"`
	OmpBin          string             `json:"omp_bin"`
	Providers       []MetaProviderInfo `json:"providers"`
}

// MetaProviderInfo is the per-provider entry in MetaResponse.
// Compatible with LoginProviderInfo on the wire.
type MetaProviderInfo struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Auth          string `json:"auth"`
	Available     bool   `json:"available"`
	Authenticated bool   `json:"authenticated"`
	EnvVar        string `json:"env_var,omitempty"`
	HelpURL       string `json:"help_url,omitempty"`
}

// ProviderProbe is the dependency the meta handler needs.
type ProviderProbe interface {
	IsConfigured(name string) bool
}

// NewMetaHandler returns the http.HandlerFunc for /api/v1/meta.
// On success (omp_bin resolved), it returns 200. When the omp
// binary cannot be resolved, it returns 503 with a stable error
// code.
//
// providers is the source of provider rows — typically the same
// LoginProvidersCache the /api/v1/login/providers handler serves.
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
