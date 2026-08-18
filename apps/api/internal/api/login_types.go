package api

// Login endpoints: provider list, login job lifecycle, SSE
// streaming, and ack roundtrip with the omp child. See
// docs/mvp/phase-1-functionality/05-pr-specs/PR-01-login-driven.md
// for the wire-format specification that drove these types.

import (
	"time"

	"github.com/lucaspdude/rocinante-harness/apps/api/internal/catalog"
)

// LoginProviderInfo mirrors catalog.LoginProviderInfo — kept as a
// type alias so existing api.LoginProviderInfo references across
// the codebase continue to compile after the type moved to catalog
// (to break the cycle with PR-02's catalog handler).
type LoginProviderInfo = catalog.LoginProviderInfo

// LoginProvidersResponse is the GET /api/v1/login/providers body.
type LoginProvidersResponse struct {
	Providers []LoginProviderInfo `json:"providers"`
	CachedAt  time.Time           `json:"cached_at"`
}

// LoginStartResponse is the 202 Accepted body for POST
// /api/v1/login/start/{provider}.
type LoginStartResponse struct {
	JobID      string `json:"job_id"`
	StreamURL  string `json:"stream_url"`
	StatusURL  string `json:"status_url"`
	ProviderID string `json:"provider_id"`
}

// LoginAck is the request body for POST /api/v1/login/{jobId}/ack.
type LoginAck struct {
	Value    string `json:"value,omitempty"`
	Canceled bool   `json:"canceled,omitempty"`
	Error    string `json:"error,omitempty"`
}

// LoginStatus is the GET /api/v1/login/{jobId}/status body.
type LoginStatus struct {
	JobID      string    `json:"job_id"`
	ProviderID string    `json:"provider_id"`
	State      string    `json:"state"`
	StartedAt  time.Time `json:"started_at"`
	EndedAt    time.Time `json:"ended_at,omitempty"`
	Error      string    `json:"error,omitempty"`
	Auth       string    `json:"auth,omitempty"`
}

// LoginStreamEvent is the SSE event payload pushed by the harness.
type LoginStreamEvent struct {
	Event string         `json:"event"`
	Data  map[string]any `json:"data"`
}
