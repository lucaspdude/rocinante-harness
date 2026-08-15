package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/lucaspdude/rocinante-harness/apps/api/internal/omp"
)

type forkRequest struct {
	AtMessageID string `json:"at_message_id"`
}

type forkResponse struct {
	ID              string `json:"id"`
	ClientRequestID string `json:"client_request_id"`
	CacheState      string `json:"cache_state"`
}

// ForkHandler creates a new session that resumes from a message id
// in an existing session. On the v1/v2 wire this is a fork command.
func ForkHandler(m *omp.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		sess := m.Get(id)
		if sess == nil {
			writeJSON(w, http.StatusNotFound, errorResponse{Code: "session_not_found"})
			return
		}
		var req forkRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, errorResponse{Code: "bad_request", Message: err.Error()})
			return
		}
		if req.AtMessageID == "" {
			writeJSON(w, http.StatusBadRequest, errorResponse{Code: "bad_request", Message: "at_message_id required"})
			return
		}
		clientID := newClientRequestID()
		frame, err := omp.BuildForkFrame(omp.ForkRequest{AtMessageID: req.AtMessageID}, clientID)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, errorResponse{Code: "bad_request", Message: err.Error()})
			return
		}
		if err := sess.SendCommand(frame); err != nil {
			writeJSON(w, http.StatusGone, errorResponse{Code: "session_closed", Message: err.Error()})
			return
		}
		// The forked session id is generated optimistically; the
		// frontend reconciles it via the SSE stream when the omp
		// emits the corresponding `forked` event.
		forkID := uuid.NewString()
		_ = time.Now()
		writeJSON(w, http.StatusCreated, forkResponse{
			ID:              forkID,
			ClientRequestID: clientID,
			CacheState:      "best-effort",
		})
	}
}
