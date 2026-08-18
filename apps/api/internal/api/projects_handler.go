package api

// Projects endpoints — PR-03 wire format. CRUD + list with session
// counts + orphans. Auth-required (same group as /sessions).

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/lucaspdude/rocinante-harness/apps/api/internal/files"
	"github.com/lucaspdude/rocinante-harness/apps/api/internal/omp"
	"github.com/lucaspdude/rocinante-harness/apps/api/internal/projects"
)

// ProjectsHandlers groups the dependencies for the /projects routes.
type ProjectsHandlers struct {
	Registry   *projects.Registry
	Sessions   *omp.Manager
	FileAccess *files.FileAccess
}

type registerProjectRequest struct {
	Path        string `json:"path"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
}

type patchProjectRequest struct {
	Path        string  `json:"path"`
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
}

type hideProjectRequest struct {
	Path   string `json:"path"`
	Hidden bool   `json:"hidden"`
}

type projectListItem struct {
	Path         string `json:"path"`
	Name         string `json:"name"`
	Description  string `json:"description,omitempty"`
	AddedAt      string `json:"added_at"`
	UpdatedAt    string `json:"updated_at"`
	Hidden       bool   `json:"hidden,omitempty"`
	SessionCount int    `json:"session_count"`
}

type orphanSessionItem struct {
	ID    string `json:"id"`
	OmpCwd string `json:"omp_cwd"`
}

type projectsListResponse struct {
	Projects []projectListItem   `json:"projects"`
	Orphans  []orphanSessionItem `json:"orphans,omitempty"`
}

// ProjectsHandler is /api/v1/projects — GET (list) and POST (register).
func (h *ProjectsHandlers) ProjectsHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.list(w, r)
	case http.MethodPost:
		h.register(w, r)
	default:
		w.Header().Set("Allow", "GET, POST")
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{
			Code: "method_not_allowed",
		})
	}
}

func (h *ProjectsHandlers) list(w http.ResponseWriter, r *http.Request) {
	if h.Registry == nil {
		writeJSON(w, http.StatusOK, projectsListResponse{Projects: []projectListItem{}})
		return
	}
	all := h.Registry.List()
	byPath := make(map[string]projectListItem, len(all))
	for _, p := range all {
		byPath[p.Path] = projectListItem{
			Path:         p.Path,
			Name:         p.Name,
			Description:  p.Description,
			AddedAt:      p.AddedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
			UpdatedAt:    p.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
			Hidden:       p.Hidden,
			SessionCount: 0,
		}
	}
	orphans := []orphanSessionItem{}
	if h.Sessions != nil {
		for _, rec := range h.Sessions.All() {
			if _, ok := byPath[rec.OmpCwd]; ok {
				byPath[rec.OmpCwd] = projectListItem{
					Path:         byPath[rec.OmpCwd].Path,
					Name:         byPath[rec.OmpCwd].Name,
					Description:  byPath[rec.OmpCwd].Description,
					AddedAt:      byPath[rec.OmpCwd].AddedAt,
					UpdatedAt:    byPath[rec.OmpCwd].UpdatedAt,
					Hidden:       byPath[rec.OmpCwd].Hidden,
					SessionCount: byPath[rec.OmpCwd].SessionCount + 1,
				}
				continue
			}
			orphans = append(orphans, orphanSessionItem{
				ID:     rec.ID,
				OmpCwd: rec.OmpCwd,
			})
		}
	}
	out := projectsListResponse{
		Projects: make([]projectListItem, 0, len(byPath)),
		Orphans:  orphans,
	}
	for _, v := range byPath {
		out.Projects = append(out.Projects, v)
	}
	sortProjects(out.Projects)
	writeJSON(w, http.StatusOK, out)
}

func (h *ProjectsHandlers) register(w http.ResponseWriter, r *http.Request) {
	var req registerProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{
			Code:    "bad_request",
			Message: err.Error(),
		})
		return
	}
	if req.Path == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{
			Code: "invalid_path",
		})
		return
	}
	got, err := h.Registry.Upsert(req.Path, req.Name, req.Description, false)
	if err != nil {
		if err == projects.ErrAlreadyRegistered {
			writeJSON(w, http.StatusConflict, errorResponse{
				Code:    "already_registered",
				Message: req.Path,
			})
			return
		}
		writeJSON(w, http.StatusInternalServerError, errorResponse{
			Code:    "registry_write_failed",
			Message: err.Error(),
		})
		return
	}
	if h.FileAccess != nil {
		_ = h.FileAccess.Allow(req.Path)
	}
	if h.Sessions != nil {
		_, _ = h.Sessions.Create(req.Path) // best-effort — fallback for legacy flows
	}
	writeJSON(w, http.StatusCreated, got)
}

// ProjectsMutationHandler is /api/v1/projects (PATCH + DELETE).
// Both use the same chi route prefix; the handler dispatches.
func (h *ProjectsHandlers) ProjectsMutationHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPatch:
		h.patch(w, r)
	case http.MethodDelete:
		h.hide(w, r)
	default:
		w.Header().Set("Allow", "PATCH, DELETE")
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{
			Code: "method_not_allowed",
		})
	}
}

func (h *ProjectsHandlers) patch(w http.ResponseWriter, r *http.Request) {
	var req patchProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{
			Code:    "bad_request",
			Message: err.Error(),
		})
		return
	}
	if req.Path == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{
			Code: "invalid_path",
		})
		return
	}
	got, err := h.Registry.Patch(req.Path, req.Name, req.Description)
	if err != nil {
		if err.Error() == "not_found" {
			writeJSON(w, http.StatusNotFound, errorResponse{Code: "not_found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, errorResponse{
			Code:    "registry_write_failed",
			Message: err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusOK, got)
}

func (h *ProjectsHandlers) hide(w http.ResponseWriter, r *http.Request) {
	var req hideProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{
			Code:    "bad_request",
			Message: err.Error(),
		})
		return
	}
	if req.Path == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{
			Code: "invalid_path",
		})
		return
	}
	if err := h.Registry.Hide(req.Path, req.Hidden); err != nil {
		if err.Error() == "not_found" {
			writeJSON(w, http.StatusNotFound, errorResponse{Code: "not_found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, errorResponse{
			Code:    "registry_write_failed",
			Message: err.Error(),
		})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// sortProjects orders by Path ASC (stable). Tiny helper so we don't
// import "sort" just for one call site.
func sortProjects(s []projectListItem) {
	for i := 1; i < len(s); i++ {
		j := i
		for j > 0 && s[j-1].Path > s[j].Path {
			s[j-1], s[j] = s[j], s[j-1]
			j--
		}
	}
}

// Unique name on the mount; chi needs different method handlers
// per sub-route. We provide PatchHandler and DeleteHandler for
// callers that want the split.
func (h *ProjectsHandlers) PatchHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPatch {
		w.Header().Set("Allow", "PATCH")
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Code: "method_not_allowed"})
		return
	}
	h.patch(w, r)
}

func (h *ProjectsHandlers) DeleteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		w.Header().Set("Allow", "DELETE")
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Code: "method_not_allowed"})
		return
	}
	h.hide(w, r)
}

// chiURLParam is a small wrapper so tests don't have to import chi.
func chiURLParam(r *http.Request, key string) string {
	return chi.URLParam(r, key)
}
