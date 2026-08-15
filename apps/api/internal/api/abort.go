package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/lucaspdude/rocinante-harness/apps/api/internal/omp"
)

type abortResponse struct {
	ClientRequestID string `json:"client_request_id"`
	Aborted         bool   `json:"aborted"`
	CacheState      string `json:"cache_state"`
}

// AbortHandler signals the omp session to abort the current run.
// Idempotent: an abort on an already-finished session returns 200.
func AbortHandler(m *omp.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		sess := m.Get(id)
		if sess == nil {
			writeJSON(w, http.StatusNotFound, errorResponse{Code: "session_not_found"})
			return
		}
		clientID := newClientRequestID()
		frame := omp.BuildAbortFrame(clientID)
		if err := sess.SendCommand(frame); err != nil {
			// Abort on a closed session is a 200 idempotent — the
			// caller's intent (stop) is already satisfied.
			writeJSON(w, http.StatusOK, abortResponse{
				ClientRequestID: clientID,
				Aborted:         true,
				CacheState:      "best-effort",
			})
			return
		}
		writeJSON(w, http.StatusOK, abortResponse{
			ClientRequestID: clientID,
			Aborted:         true,
			CacheState:      "best-effort",
		})
	}
}
