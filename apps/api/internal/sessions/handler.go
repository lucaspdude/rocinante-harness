package sessions

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strconv"

	"github.com/go-chi/chi/v5"
)

// ReplayResponse is the JSON shape of GET /api/v1/sessions/{id}/messages.
// `entries` carries every persisted line since `since`; the web client
// re-seeds its reducer from these. An empty `entries` means a fresh
// session (no history yet).
type ReplayResponse struct {
	ID      string  `json:"id"`
	Entries []Entry `json:"entries"`
}

// ReplayHandler returns an http.HandlerFunc that replays the persisted
// JSONL log for the session. The optional ?since=N query parameter
// skips the first N lines (used by clients resuming from an offset).
//
// Returns 200 with entries (possibly empty) when the log exists;
// 200 with empty entries when the session has no history yet;
// 404 when the session id is malformed (empty) — chi refuses to
// route an empty {id} segment, so this is a defensive guard.
func ReplayHandler(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		if id == "" {
			writeError(w, http.StatusNotFound, "session_not_found")
			return
		}
		since := 0
		if raw := r.URL.Query().Get("since"); raw != "" {
			n, err := strconv.Atoi(raw)
			if err != nil || n < 0 {
				writeError(w, http.StatusBadRequest, "bad_since")
				return
			}
			since = n
		}
		entries, err := store.Replay(id, since)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				writeJSON(w, http.StatusOK, ReplayResponse{ID: id, Entries: nil})
				return
			}
			writeError(w, http.StatusInternalServerError, "replay_failed")
			return
		}
		if entries == nil {
			entries = []Entry{}
		}
		writeJSON(w, http.StatusOK, ReplayResponse{ID: id, Entries: entries})
	}
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, code string) {
	writeJSON(w, status, map[string]string{"code": code})
}
