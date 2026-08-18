package api

import (
	"bufio"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/lucaspdude/rocinante-harness/apps/api/internal/omp"
)

type sessionRequest struct {
	OmpCwd     string `json:"omp_cwd"`
	ProjectPath string `json:"project_path,omitempty"`
}

type errorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message,omitempty"`
}

// CreateSessionHandler spawns a new omp session.
func CreateSessionHandler(m *omp.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req sessionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, errorResponse{Code: "bad_request", Message: err.Error()})
			return
		}
		cwd := req.OmpCwd
		if cwd == "" {
			cwd = req.ProjectPath
		}
		rec, err := m.Create(cwd)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errorResponse{Code: "spawn_failed", Message: err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, rec)
	}
}

// StreamSessionHandler relays the omp stdout stream as SSE.
func StreamSessionHandler(m *omp.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		sess := m.Get(id)
		if sess == nil {
			writeJSON(w, http.StatusNotFound, errorResponse{Code: "session_not_found"})
			return
		}
		flusher, ok := w.(http.Flusher)
		if !ok {
			writeJSON(w, http.StatusInternalServerError, errorResponse{Code: "no_flusher"})
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("X-Accel-Buffering", "no")
		w.WriteHeader(http.StatusOK)
		stream := sess.Reader()
		scanner := bufio.NewScanner(stream)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			line := scanner.Bytes()
			if _, err := w.Write([]byte("data: ")); err != nil {
				return
			}
			if _, err := w.Write(line); err != nil {
				return
			}
			if _, err := w.Write([]byte("\n\n")); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

// CloseSessionHandler terminates a session.
func CloseSessionHandler(m *omp.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		if err := m.Close(id); err != nil {
			writeJSON(w, http.StatusNotFound, errorResponse{Code: "session_not_found"})
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		_ = errors.New("encode: " + err.Error())
	}
}
