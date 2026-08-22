package api

// PR-03 sessions surface + is_orphan + project_path (F7 review
// followup). The /api/v1/sessions POST response now includes:
//
//   is_orphan      — true when no project is registered at the
//                     requested cwd; false when the project registry
//                     has it.
//   project_path   — the registered project path (== cwd when
//                     the project exists, "" when orphan).
//
// The cwd may be set via either `omp_cwd` (legacy) or
// `project_path` (preferred). Project registration happens through
// POST /api/v1/projects.
//
// PR-2 (phase 6) extends POST /api/v1/sessions/{id}/prompt and
// GET /api/v1/sessions/{id}/events to mirror writes into a JSONL
// log via internal/sessions.Store. The web replays this log on
// mount to rehydrate the chat after a reload.

import (
	"bufio"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/lucaspdude/rocinante-harness/apps/api/internal/omp"
	"github.com/lucaspdude/rocinante-harness/apps/api/internal/sessions"
)

// sessionRecorder is the seam StreamSessionHandler and the prompt
// handler use to mirror writes to the JSONL log. The concrete
// *sessions.Store satisfies this interface; the interface lives
// here to keep the dependency direction outward (api → sessions)
// without dragging the store into other packages.
type sessionRecorder interface {
	Append(id string, entry sessions.Entry)
}

type sessionRequest struct {
	OmpCwd      string `json:"omp_cwd"`
	ProjectPath string `json:"project_path,omitempty"`
}

// sessionResponse wraps omp.SessionRecord with the extra
// is_orphan + project_path fields.
type sessionResponse struct {
	ID              string `json:"id"`
	OmpCwd          string `json:"omp_cwd"`
	Cwd             string `json:"cwd"`
	ProjectPath     string `json:"project_path,omitempty"`
	IsOrphan        bool   `json:"is_orphan"`
	CreatedAt       string `json:"created_at"`
	ProtocolVersion int    `json:"protocol_version"`
	State           string `json:"state"`
	LastSeenAt      string `json:"last_seen_at"`
}

type errorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message,omitempty"`
}

// SessionHandlerDeps groups the optional persistence seam around
// the existing session handler constructors. Recorder is the JSONL
// store from internal/sessions. nil disables persistence (kept for
// tests that don't need a share dir).
type SessionHandlerDeps struct {
	Recorder sessionRecorder
}

// ProjectsLister is the seam CreateSessionHandler needs to know
// whether the requested cwd is a registered project. The router
// wires this to *projects.Registry; nil is acceptable (every
// session becomes orphan).
type ProjectsLister interface {
	IsRegistered(path string) bool
}

// CreateSessionHandler spawns a new omp session. It marks the
// session as orphan or not based on whether the cwd matches a
// registered project in the ProjectRegistry (F7).
func CreateSessionHandler(
	m *omp.Manager,
	registry ProjectsLister,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req sessionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, errorResponse{
				Code:    "bad_request",
				Message: err.Error(),
			})
			return
		}
		cwd := req.OmpCwd
		if cwd == "" {
			cwd = req.ProjectPath
		}
		rec, err := m.Create(cwd)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errorResponse{
				Code:    "spawn_failed",
				Message: err.Error(),
			})
			return
		}
		// F7: cross-reference with the project registry.
		isOrphan := true
		projectPath := ""
		if registry != nil && cwd != "" && registry.IsRegistered(cwd) {
			isOrphan = false
			projectPath = cwd
		}
		writeJSON(w, http.StatusCreated, sessionResponse{
			ID:              rec.ID,
			OmpCwd:          rec.OmpCwd,
			Cwd:             rec.Cwd,
			ProjectPath:     projectPath,
			IsOrphan:        isOrphan,
			CreatedAt:       rec.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
			ProtocolVersion: rec.ProtocolVersion,
			State:           rec.State,
			LastSeenAt:      rec.LastSeenAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		})
	}
}

// PromptHandlerWithRecorder is the PR-2 variant of PromptHandler
// (defined in prompt.go). When rec is non-nil, the user message is
// mirrored into the JSONL log alongside the per-frame SSE writes
// performed by StreamSessionHandlerWithRecorder. Pass nil to keep
// the pre-PR-2 behavior (no persistence).
func PromptHandlerWithRecorder(m *omp.Manager, rec sessionRecorder) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		sess := m.Get(id)
		if sess == nil {
			writeJSON(w, http.StatusNotFound, errorResponse{Code: "session_not_found"})
			return
		}
		var req promptRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, errorResponse{Code: "bad_request", Message: err.Error()})
			return
		}
		if req.Text == "" {
			writeJSON(w, http.StatusBadRequest, errorResponse{Code: "empty_text", Message: "text required"})
			return
		}
		clientID := newClientRequestID()
		frame, err := omp.BuildPromptFrame(omp.PromptRequest{
			Text:          req.Text,
			Model:         cmpOr(req.Model, req.ModelID),
			ModelRole:     req.ModelRole,
			ThinkingLevel: req.ThinkingLevel,
		}, clientID)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, errorResponse{Code: "bad_request", Message: err.Error()})
			return
		}
		if err := sess.SendCommand(frame); err != nil {
			writeJSON(w, http.StatusGone, errorResponse{Code: "session_closed", Message: err.Error()})
			return
		}
		if rec != nil {
			model := cmpOr(req.Model, req.ModelID)
			payload, _ := json.Marshal(map[string]any{
				"id":        clientID,
				"role":      "user",
				"content":   req.Text,
				"createdAt": nowISO(),
				"model":     model,
			})
			rec.Append(id, sessions.Entry{Kind: "message", Message: payload})
		}
		writeJSON(w, http.StatusOK, promptResponse{
			ClientRequestID: clientID,
			Queued:          true,
			CacheState:      "best-effort",
		})
	}
}

// StreamSessionHandler relays the omp stdout stream as SSE.
func StreamSessionHandler(m *omp.Manager) http.HandlerFunc {
	return StreamSessionHandlerWithRecorder(m, nil)
}

// StreamSessionHandlerWithRecorder is the PR-2 variant that mirrors
// every SSE frame into the JSONL log so a reconnect can replay the
// tail. Pass nil to disable persistence.
func StreamSessionHandlerWithRecorder(m *omp.Manager, rec sessionRecorder) http.HandlerFunc {
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
			if rec != nil && len(line) > 0 {
				rec.Append(id, sessions.Entry{Kind: "frame", Frame: append(json.RawMessage(nil), line...)})
			}
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

// nowISO returns the current wall-clock time in RFC3339 with
// milliseconds. The web renders createdAt from this same format
// so replay matches.
func nowISO() string {
	return time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
}
