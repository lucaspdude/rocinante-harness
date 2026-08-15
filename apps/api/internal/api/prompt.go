package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/lucaspdude/rocinante-harness/apps/api/internal/omp"
)

type promptRequest struct {
	Text          string `json:"text"`
	Model         string `json:"model,omitempty"`
	ModelRole     string `json:"model_role,omitempty"`
	ThinkingLevel string `json:"thinking_level,omitempty"`
}

type promptResponse struct {
	ClientRequestID string `json:"client_request_id"`
	Queued          bool   `json:"queued"`
	CacheState      string `json:"cache_state"`
}

// PromptHandler accepts a prompt and writes the corresponding NDJSON
// frame to the session's stdin.
func PromptHandler(m *omp.Manager) http.HandlerFunc {
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
			Model:         req.Model,
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
		writeJSON(w, http.StatusOK, promptResponse{
			ClientRequestID: clientID,
			Queued:          true,
			CacheState:      "best-effort",
		})
	}
}
