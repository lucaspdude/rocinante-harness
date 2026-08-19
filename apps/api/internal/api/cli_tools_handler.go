package api

// PR-06: CLI install + device-code login endpoints.
//
// Five routes under /api/v1/cli-tools/{id}/*, all auth-protected
// (PR-06 spec requires auth since they read user-level config):
//
//	POST   /api/v1/cli-tools/{id}/install               → 202 { jobId, pid }
//	GET    /api/v1/cli-tools/{id}/install/{jobId}/stream → SSE log/status/end
//	POST   /api/v1/cli-tools/{id}/login/start           → 202 { jobId, authUrl, authCode }
//	POST   /api/v1/cli-tools/{id}/login/{jobId}/ack     → 204 (body { value })
//	GET    /api/v1/cli-tools/{id}/status                → 200 { installed, authenticated, version, account, detail }
//
// The handlers are thin wrappers around *clitools.Manager. The
// SSE handler (cli_tools_stream.go) holds the connection open
// until the job terminates, replaying buffered lines and
// emitting log/status/end events as the runner reports.

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/lucaspdude/rocinante-harness/apps/api/internal/clitools"
)

// CliToolsHandler groups the deps the cli-tools endpoints
// need. Mirrors the CloneHandlers / LoginHandlers pattern.
type CliToolsHandler struct {
	Manager *clitools.Manager
}

// InstallStartResponse is the 202 body of
// POST /api/v1/cli-tools/{id}/install.
type InstallStartResponse struct {
	JobID string `json:"jobId"`
	Pid   int    `json:"pid"`
}

// CliLoginStartResponse is the 202 body of
// POST /api/v1/cli-tools/{id}/login/start. authUrl/authCode
// are filled by the runner's regex pass; they may be empty in
// the first few hundred milliseconds if the provider hasn't
// printed its prompt yet — the web client polls the status
// stream or just retries after a moment.
type CliLoginStartResponse struct {
	JobID    string `json:"jobId"`
	AuthURL  string `json:"authUrl,omitempty"`
	AuthCode string `json:"authCode,omitempty"`
}

// InstallHandler is POST /api/v1/cli-tools/{id}/install.
func (h *CliToolsHandler) InstallHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Code: "method_not_allowed"})
		return
	}
	cliID := chi.URLParam(r, "id")
	if cliID == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Code: "missing_id"})
		return
	}
	job, err := h.Manager.InstallStart(r.Context(), cliID)
	if err != nil {
		switch {
		case errors.Is(err, clitools.ErrUnknownCli):
			writeJSON(w, http.StatusNotFound, errorResponse{Code: "unknown_cli", Message: cliID})
		case errors.Is(err, clitools.ErrUnsupportedPlatform):
			writeJSON(w, http.StatusUnprocessableEntity, errorResponse{
				Code:    "unsupported_platform",
				Message: "install yourself; this panel will detect it",
			})
		default:
			writeJSON(w, http.StatusInternalServerError, errorResponse{Code: "spawn_failed", Message: err.Error()})
		}
		return
	}
	writeJSON(w, http.StatusAccepted, InstallStartResponse{JobID: job.ID, Pid: job.Pid})
}

// LoginStartHandler is POST /api/v1/cli-tools/{id}/login/start.
func (h *CliToolsHandler) LoginStartHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Code: "method_not_allowed"})
		return
	}
	cliID := chi.URLParam(r, "id")
	if cliID == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Code: "missing_id"})
		return
	}
	job, err := h.Manager.LoginStart(r.Context(), cliID)
	if err != nil {
		switch {
		case errors.Is(err, clitools.ErrUnknownCli):
			writeJSON(w, http.StatusNotFound, errorResponse{Code: "unknown_cli", Message: cliID})
		case errors.Is(err, clitools.ErrUnsupportedPlatform):
			writeJSON(w, http.StatusUnprocessableEntity, errorResponse{
				Code:    "no_login_cmd",
				Message: "no LoginCmd for this provider",
			})
		default:
			writeJSON(w, http.StatusInternalServerError, errorResponse{Code: "spawn_failed", Message: err.Error()})
		}
		return
	}
	writeJSON(w, http.StatusAccepted, CliLoginStartResponse{
		JobID:    job.ID,
		AuthURL:  job.AuthURL,
		AuthCode: job.AuthCode,
	})
}

// AckRequest is the body of POST /api/v1/cli-tools/{id}/login/{jobId}/ack.
type AckRequest struct {
	Value string `json:"value"`
}

// AckLoginHandler is POST /api/v1/cli-tools/{id}/login/{jobId}/ack.
func (h *CliToolsHandler) AckLoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Code: "method_not_allowed"})
		return
	}
	jobID := chi.URLParam(r, "jobId")
	if jobID == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Code: "missing_job_id"})
		return
	}
	var req AckRequest
	if r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, errorResponse{Code: "bad_request", Message: err.Error()})
			return
		}
	}
	if err := h.Manager.LoginAck(jobID, req.Value); err != nil {
		if errors.Is(err, clitools.ErrJobNotFound) {
			writeJSON(w, http.StatusNotFound, errorResponse{Code: "job_not_found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, errorResponse{Code: "ack_failed", Message: err.Error()})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// StatusHandler is GET /api/v1/cli-tools/{id}/status.
func (h *CliToolsHandler) StatusHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Code: "method_not_allowed"})
		return
	}
	cliID := chi.URLParam(r, "id")
	if cliID == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Code: "missing_id"})
		return
	}
	resp := h.Manager.Status(r.Context(), cliID)
	writeJSON(w, http.StatusOK, resp)
}

// ListHandler is GET /api/v1/cli-tools. Optional convenience
// for the web panel to enumerate supported providers without
// hard-coding the list.
func (h *CliToolsHandler) ListHandler(w http.ResponseWriter, r *http.Request) {
	type entry struct {
		ID          string `json:"id"`
		DisplayName string `json:"displayName"`
		HelpText    string `json:"helpText"`
	}
	out := make([]entry, 0, len(clitools.CLIS))
	for _, id := range clitools.List() {
		if spec, ok := clitools.GetSpec(id); ok {
			out = append(out, entry{ID: spec.ID, DisplayName: spec.DisplayName, HelpText: spec.HelpText})
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"clis": out})
}