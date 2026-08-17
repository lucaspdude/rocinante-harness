package omp

import (
	"encoding/json"
	"net/http"
)

// MetaResponse is the JSON body of /api/v1/meta.
type MetaResponse struct {
	APIVersion      string `json:"api_version"`
	OmpVersion      string `json:"omp_version"`
	ProtocolVersion int    `json:"protocol_version"`
	OmpBin          string `json:"omp_bin"`
	// Providers reports whether each provider's API key is
	// configured. A provider is "configured" when EITHER the
	// matching env var is set in the api's process OR the
	// keystore on disk has an entry for it. The actual key
	// value is never returned — only the booleans.
	Providers ProviderStatus `json:"providers"`
}

// ProviderStatus is the set of providers the api recognizes.
// Each field is true when the corresponding provider has a
// configured key. The field name is the canonical provider id
// (matches the keystore's ProviderName string).
type ProviderStatus struct {
	Anthropic  bool `json:"anthropic"`
	OpenAI     bool `json:"openai"`
	Gemini     bool `json:"gemini"`
	OpenRouter bool `json:"openrouter"`
	Minimax    bool `json:"minimax"`
}

// ProviderProbe is the dependency the meta handler needs to
// answer the providers checklist. The router wires this from
// the keystore + os.Environ(); tests can use a stub.
type ProviderProbe interface {
	IsConfigured(name string) bool
}

// NewMetaHandler returns the http.HandlerFunc for /api/v1/meta.
// On success (omp_bin resolved), it returns 200. When the omp
// binary cannot be resolved, it returns 503 with a stable error
// code.
func NewMetaHandler(loader Loader, apiVersion string, probe ProviderProbe) http.HandlerFunc {
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
		status := ProviderStatus{
			Anthropic:  probe.IsConfigured("anthropic"),
			OpenAI:     probe.IsConfigured("openai"),
			Gemini:     probe.IsConfigured("gemini"),
			OpenRouter: probe.IsConfigured("openrouter"),
			Minimax:    probe.IsConfigured("minimax"),
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(MetaResponse{
			APIVersion:      apiVersion,
			OmpVersion:      version,
			ProtocolVersion: protocol,
			OmpBin:          bin,
			Providers:       status,
		})
	}
}
