package api

// PR-06: SSE stream for the install job.
//
//	GET /api/v1/cli-tools/{id}/install/{jobId}/stream
//
// Emits three event types:
//
//	event: log     data: {"line": "<captured stdout/stderr line>"}
//	event: status  data: {"status": "running|done|failed", "exitCode": <int|null>}
//	event: end     data: {"status": "done|failed", "exitCode": <int|null>}
//
// On connect, the handler replays the buffered lines (up to
// ringCap) as `log` events so a slow/late subscriber still
// sees the install progress from the start. Heartbeats every
// 15 seconds (mirrors /api/v1/login/.../stream and the chat
// SSE) keep idle connections from being reaped by proxies.

import (
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/lucaspdude/rocinante-harness/apps/api/internal/clitools"
)

// InstallStreamHandler is GET
// /api/v1/cli-tools/{id}/install/{jobId}/stream.
func (h *CliToolsHandler) InstallStreamHandler(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "jobId")
	if jobID == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Code: "missing_job_id"})
		return
	}
	job := h.Manager.Jobs.Get(jobID)
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

	// Snapshot the buffered lines once on connect and replay
	// them. We re-snapshot on each ping to learn about new
	// lines; the snapshot is a copy, so we don't need a
	// long-held lock.
	initial := job.Snapshot()
	for _, line := range initial.Lines {
		if !writeSSE(w, "log", map[string]any{
			"line":   line,
			"job_id": job.ID,
		}) {
			return
		}
	}
	flusher.Flush()

	// If the job already finished by the time we connected,
	// emit the terminal event and return.
	if initial.Status != clitools.JobRunning {
		_ = writeSSE(w, "end", map[string]any{
			"job_id":    initial.ID,
			"status":    string(initial.Status),
			"exit_code": intPtr(initial.ExitCode),
		})
		flusher.Flush()
		return
	}

	// Otherwise subscribe and pump until the job finishes.
	ping, unsub := job.Subscribe()
	defer unsub()
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	// lastIndex tracks how many lines we've already replayed
	// so each subsequent event reports only the deltas. The
	// monitor calls job.notify() after each new line.
	lastIndex := len(initial.Lines)
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
		case <-ping:
			snap := job.Snapshot()
			// Drain new lines since lastIndex.
			newLines := snap.Lines[lastIndex:]
			for _, line := range newLines {
				if !writeSSE(w, "log", map[string]any{
					"line":   line,
					"job_id": snap.ID,
				}) {
					return
				}
			}
			lastIndex += len(newLines)
			if snap.Status != clitools.JobRunning {
				_ = writeSSE(w, "end", map[string]any{
					"job_id":    snap.ID,
					"status":    string(snap.Status),
					"exit_code": intPtr(snap.ExitCode),
				})
				flusher.Flush()
				return
			}
			// status event reports the current running
			// state so the UI can show a spinner without
			// re-reading the buffer.
			_ = writeSSE(w, "status", map[string]any{
				"job_id": snap.ID,
				"status": string(snap.Status),
			})
			flusher.Flush()
		}
	}
}

func intPtr(p *int) any {
	if p == nil {
		return nil
	}
	return *p
}