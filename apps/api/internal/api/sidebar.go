package api

import (
	"encoding/json"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/lucaspdude/rocinante-harness/apps/api/internal/omp"
)

// SessionListItem is the wire form of a session in the sidebar
// response. omp_cwd is the canonical "folder" key the front groups by.
type SessionListItem struct {
	ID         string    `json:"id"`
	OmpCwd     string    `json:"omp_cwd"`
	Title      string    `json:"title"`
	CreatedAt  time.Time `json:"created_at"`
	LastSeenAt time.Time `json:"last_seen_at"`
	State      string    `json:"state"`
}

type SessionGroup struct {
	OmpCwd   string            `json:"omp_cwd"`
	Sessions []SessionListItem `json:"sessions"`
}

type SessionListResponse struct {
	Groups []SessionGroup `json:"groups"`
}

type SessionTitle struct {
	Title string `json:"title"`
}

// titleKey is a small in-memory store of session titles keyed by
// session id. P14 will move this to SQLite.
type titleKey struct {
	mu     sync.Mutex
	titles map[string]string
}

func newTitleStore() *titleKey {
	return &titleKey{titles: make(map[string]string)}
}

func (t *titleKey) get(id string) string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.titles[id]
}

func (t *titleKey) set(id, title string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.titles[id] = title
}

// SessionsListHandler returns the grouped session list.
func SessionsListHandler(m *omp.Manager, t *titleKey) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		recs := m.All()
		groups := make(map[string][]SessionListItem)
		for _, rec := range recs {
			item := SessionListItem{
				ID:         rec.ID,
				OmpCwd:     rec.OmpCwd,
				Title:      t.get(rec.ID),
				CreatedAt:  rec.CreatedAt,
				LastSeenAt: rec.LastSeenAt,
				State:      rec.State,
			}
			groups[rec.OmpCwd] = append(groups[rec.OmpCwd], item)
		}
		out := SessionListResponse{Groups: make([]SessionGroup, 0, len(groups))}
		for cwd, list := range groups {
			sort.Slice(list, func(i, j int) bool {
				return list[i].CreatedAt.After(list[j].CreatedAt)
			})
			out.Groups = append(out.Groups, SessionGroup{OmpCwd: cwd, Sessions: list})
		}
		sort.Slice(out.Groups, func(i, j int) bool {
			if len(out.Groups[i].Sessions) == 0 || len(out.Groups[j].Sessions) == 0 {
				return false
			}
			return out.Groups[i].Sessions[0].CreatedAt.After(out.Groups[j].Sessions[0].CreatedAt)
		})
		writeJSON(w, http.StatusOK, out)
	}
}

// SessionTitleHandler sets a session's title.
func SessionTitleHandler(m *omp.Manager, t *titleKey) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		if m.Get(id) == nil {
			writeJSON(w, http.StatusNotFound, errorResponse{Code: "session_not_found"})
			return
		}
		var req SessionTitle
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, errorResponse{Code: "bad_request", Message: err.Error()})
			return
		}
		t.set(id, req.Title)
		writeJSON(w, http.StatusOK, map[string]string{"title": req.Title})
	}
}

// SessionDeleteHandler closes a session (idempotent 204).
func SessionDeleteHandler(m *omp.Manager, t *titleKey) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		if err := m.Close(id); err != nil {
			writeJSON(w, http.StatusNotFound, errorResponse{Code: "session_not_found"})
			return
		}
		t.set(id, "")
		w.WriteHeader(http.StatusNoContent)
	}
}
