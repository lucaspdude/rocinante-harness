package api

// Login endpoints: provider list, login job lifecycle, SSE
// streaming, and ack roundtrip with the omp child. See
// docs/mvp/phase-1-functionality/05-pr-specs/PR-01-login-driven.md
// for the wire-format specification that drove these types.

import "time"

// LoginProviderInfo is the wire form for GET /api/v1/login/providers.
// The id matches the canonical omp provider id; auth "oauth" means
// the provider supports OAuth (sign in via browser); auth
// "paste-key" means the user pastes an API key into the harness
// form directly. "keyless" means omp talks to a local engine
// (ollama/lmstudio) and no auth is required.
type LoginProviderInfo struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Auth          string `json:"auth"` // "oauth" | "paste-key" | "keyless"
	Available     bool   `json:"available"`
	Authenticated bool   `json:"authenticated"`
	EnvVar        string `json:"env_var,omitempty"`
	HelpURL       string `json:"help_url,omitempty"`
}

// LoginProvidersResponse is the GET /api/v1/login/providers body.
type LoginProvidersResponse struct {
	Providers []LoginProviderInfo `json:"providers"`
	CachedAt  time.Time           `json:"cached_at"`
}

// LoginStartResponse is the 202 Accepted body for POST
// /api/v1/login/start/{provider}. The jobId is the SSE subscription
// handle. status_url is a convenience for clients that don't want
// to build the stream URL themselves.
type LoginStartResponse struct {
	JobID      string `json:"job_id"`
	StreamURL  string `json:"stream_url"`
	StatusURL  string `json:"status_url"`
	ProviderID string `json:"provider_id"`
}

// LoginAck is the request body for POST /api/v1/login/{jobId}/ack.
// The browser sends back the value it collected from the user (a
// pasted API key, an OAuth response, a device-code confirmation,
// etc). The exact shape mirrors docs/rpc.md:621-628. We map 1:1 to
// the omp child's extension_ui_response frame.
type LoginAck struct {
	Value    string `json:"value,omitempty"`
	Canceled bool   `json:"canceled,omitempty"`
	Error    string `json:"error,omitempty"`
}

// LoginStatus is the GET /api/v1/login/{jobId}/status body. State
// is "running" | "complete" | "failed" | "expired". On success the
// Provider ID + Auth route capture what just authenticated.
type LoginStatus struct {
	JobID      string    `json:"job_id"`
	ProviderID string    `json:"provider_id"`
	State      string    `json:"state"`
	StartedAt  time.Time `json:"started_at"`
	EndedAt    time.Time `json:"ended_at,omitempty"`
	Error      string    `json:"error,omitempty"`
	Auth       string    `json:"auth,omitempty"`
}

// LoginStreamEvent is the SSE event payload pushed by the harness
// as it proxies omp's extension_ui_request frames to the browser.
// Wire shape mirrors the format documented in §3 of
// docs/mvp/phase-1-functionality/05-pr-specs/README.md.
type LoginStreamEvent struct {
	Event string         `json:"event"`
	Data  map[string]any `json:"data"`
}
