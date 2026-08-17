package omp

import (
	"encoding/json"
	"net/http"
	"os"
)

// MetaResponse is the JSON body of /api/v1/meta.
type MetaResponse struct {
	APIVersion      string `json:"api_version"`
	OmpVersion      string `json:"omp_version"`
	ProtocolVersion int    `json:"protocol_version"`
	OmpBin          string `json:"omp_bin"`
	// Providers reports whether each provider's API key is set
	// in the api's environment. The api doesn't read the key
	// itself; it just inherits os.Environ() to the omp subprocess.
	// We report the booleans so the web UI can render a checklist
	// without ever handling the key.
	Providers ProviderStatus `json:"providers"`
}

// ProviderStatus is the set of provider API keys the api
// recognizes. Each field is true when the matching env var is
// set to a non-empty value in the api process; otherwise false.
// The key value is never copied into the JSON.
type ProviderStatus struct {
	Anthropic        bool `json:"anthropic"`
	OpenAI           bool `json:"openai"`
	Gemini           bool `json:"gemini"`
	OpenRouter       bool `json:"openrouter"`
	MinimaxTokenPlan bool `json:"minimax_token_plan"`
}

// envHas reports whether name is set in the api's environment
// to a non-empty value. We use the result for the /api/v1/meta
// "providers" checklist; the key value is never returned.
func envHas(name string) bool {
	return os.Getenv(name) != ""
}

// detectProviders returns the set of provider flags the web UI
// shows in its "Providers" tab. The web UI can't read or write
// the env vars itself; this is a read-only signal.
func detectProviders() ProviderStatus {
	return ProviderStatus{
		Anthropic:        envHas("ANTHROPIC_API_KEY"),
		OpenAI:           envHas("OPENAI_API_KEY"),
		Gemini:           envHas("GEMINI_API_KEY"),
		OpenRouter:       envHas("OPENROUTER_API_KEY"),
		MinimaxTokenPlan: envHas("MINIMAX_TOKEN_PLAN_API_KEY"),
	}
}

// NewMetaHandler returns the http.HandlerFunc for /api/v1/meta.
// On success (omp_bin resolved), it returns 200. When the omp
// binary cannot be resolved, it returns 503 with a stable error
// code.
func NewMetaHandler(loader Loader, apiVersion string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		bin := loader.OmpBin()
		if bin == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"code":    "omp_not_found",
				"message": "omp binary not found on PATH or $OMP_BIN",
			})
			return
		}
		protocol, version := loader.OmpVersion()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(MetaResponse{
			APIVersion:      apiVersion,
			OmpVersion:      version,
			ProtocolVersion: protocol,
			OmpBin:          bin,
			Providers:       detectProviders(),
		})
	}
}
