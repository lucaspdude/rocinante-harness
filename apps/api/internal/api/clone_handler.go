package api

// PR-04: Project clone endpoint. Three routes under
// /api/v1/projects/clone:
//   POST   /api/v1/projects/clone                   (start, 202 Accepted)
//   GET    /api/v1/projects/clone/{jobId}/stream   (SSE events)
//   GET    /api/v1/projects/clone/{jobId}/status   (terminal state)

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/lucaspdude/rocinante-harness/apps/api/internal/files"
	"github.com/lucaspdude/rocinante-harness/apps/api/internal/projects"
)

// CloneHandlers groups the deps for clone endpoints.
type CloneHandlers struct {
	Jobs       *projects.CloneJobs
	Registry   *projects.Registry
	FileAccess *files.FileAccess
	GitBin     string // path to git binary; defaults to "git"
}

// CloneStartResponse is the 202 body.
type CloneStartResponse struct {
	JobID     string `json:"job_id"`
	StreamURL string `json:"stream_url"`
	StatusURL string `json:"status_url"`
	URL       string `json:"url"`
	Target    string `json:"target"`
}

// CloneStartHandler is POST /api/v1/projects/clone. Auth-required.
func (h *CloneHandlers) CloneStartHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Code: "method_not_allowed"})
		return
	}
	var req projects.CloneRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Code: "bad_request", Message: err.Error()})
		return
	}
	if err := req.Validate(); err != nil {
		switch err.Error() {
		case "ssh_keys_missing":
			writeJSON(w, http.StatusUnprocessableEntity, errorResponse{
				Code:    "ssh_keys_missing",
				Message: "SSH URLs require configuring SSH keys first (Phase 3)",
			})
		case "invalid_url", "invalid_folder_name":
			writeJSON(w, http.StatusUnprocessableEntity, errorResponse{
				Code: err.Error(),
			})
		default:
			writeJSON(w, http.StatusBadRequest, errorResponse{Code: "bad_request", Message: err.Error()})
		}
		return
	}
	job := h.Jobs.NewJob(req)
	bin := h.GitBin
	if bin == "" {
		bin = "git"
	}
	go job.Run(r.Context(), bin, h.Registry, h.FileAccess, nil)

	target := req.ParentPath + "/" + req.FolderName
	writeJSON(w, http.StatusAccepted, CloneStartResponse{
		JobID:     job.ID,
		StreamURL: fmt.Sprintf("/api/v1/projects/clone/%s/stream", job.ID),
		StatusURL: fmt.Sprintf("/api/v1/projects/clone/%s/status", job.ID),
		URL:       req.URL,
		Target:    target,
	})
}

// CloneStreamHandler is GET /api/v1/projects/clone/{jobId}/stream.
func (h *CloneHandlers) CloneStreamHandler(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "jobId")
	job := h.Jobs.Get(jobID)
	if job == nil {
		writeJSON(w, http.StatusNotFound, errorResponse{Code: "job_not_found"})
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Code: "no_flusher"})
		return
	}
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	past, unsub := job.Subscribe()
	defer unsub()
	for _, ev := range past {
		if !writeSSE(w, ev.Event, ev.Data) {
			return
		}
	}
	flusher.Flush()

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			if _, err := fmt.Fprint(w, ":keepalive\n\n"); err != nil {
				return
			}
			flusher.Flush()
			continue
		default:
		}
		cur := h.Jobs.Get(jobID)
		if cur == nil {
			return
		}
		snap := cur.Snapshot()
		if snap.State != "running" {
			_ = writeSSE(w, "status", map[string]any{
				"job_id": snap.JobID,
				"state":  snap.State,
			})
			flusher.Flush()
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// CloneStatusResponse is GET /api/v1/projects/clone/{jobId}/status.
type CloneStatusResponse struct {
	JobID     string `json:"job_id"`
	State     string `json:"state"`
	URL       string `json:"url"`
	Target    string `json:"target"`
	CreatedAt string `json:"created_at"`
	EndedAt   string `json:"ended_at,omitempty"`
	Error     string `json:"error,omitempty"`
}

// CloneStatusHandler is GET /api/v1/projects/clone/{jobId}/status.
func (h *CloneHandlers) CloneStatusHandler(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "jobId")
	job := h.Jobs.Get(jobID)
	if job == nil {
		writeJSON(w, http.StatusNotFound, errorResponse{Code: "job_not_found"})
		return
	}
	snap := job.Snapshot()
	resp := CloneStatusResponse{
		JobID:     jobID,
		State:     snap.State,
		URL:       snap.URL,
		Target:    snap.ParentPath + "/" + snap.FolderName,
		CreatedAt: snap.CreatedAt.UTC().Format(time.RFC3339),
		Error:     snap.Error,
	}
	if !snap.EndedAt.IsZero() {
		resp.EndedAt = snap.EndedAt.UTC().Format(time.RFC3339)
	}
	writeJSON(w, http.StatusOK, resp)
}

// ensure strings import is used in trimmed builds.
var _ = strings.HasPrefix
