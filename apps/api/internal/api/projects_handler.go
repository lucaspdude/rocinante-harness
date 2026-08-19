package api

// Projects endpoints — PR-03 wire format. CRUD + list with session
// counts + orphans. Auth-required (same group as /sessions).

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"

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
	// Home is the value ExpandHome resolves "~" against on POST.
	Home string
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
	req.Path = files.ExpandHome(req.Path, h.Home)
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
// Projects bulk operations — PR-07.
//
// POST /api/v1/projects/bulk
//   {op:"archive"|"delete", paths:[string], confirmPath?:string}
//
// Archive: Hide(path, true) for each entry; collect per-path errors.
// Delete:  require confirmPath matching one of the paths, then
//          os.RemoveAll(path) + Hide(path, true). Destructive —
//          confirmPath is the safety check against typos.
//
// Errors are collected per path and returned with 200; the request
// as a whole only fails 4xx for shape problems (empty paths,
// unknown op, traversal segments, missing/mismatched confirmPath
// on delete). Non-2xx responses use the standard errorResponse.

type bulkProjectsRequest struct {
	Op          string   `json:"op"`
	Paths       []string `json:"paths"`
	ConfirmPath string   `json:"confirmPath,omitempty"`
}

type bulkProjectError struct {
	Path    string `json:"path"`
	Code    string `json:"code"`
	Message string `json:"message,omitempty"`
}

type bulkProjectsResponse struct {
	Archived int                `json:"archived,omitempty"`
	Deleted  int                `json:"deleted,omitempty"`
	Errors   []bulkProjectError `json:"errors,omitempty"`
}

// BulkHandler is the POST /api/v1/projects/bulk endpoint.
func (h *ProjectsHandlers) BulkHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Code: "method_not_allowed"})
		return
	}
	var req bulkProjectsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{
			Code:    "bad_request",
			Message: err.Error(),
		})
		return
	}
	if len(req.Paths) == 0 {
		writeJSON(w, http.StatusBadRequest, errorResponse{
			Code:    "invalid_query",
			Message: "paths must be non-empty",
		})
		return
	}
	switch req.Op {
	case "archive":
		// Fall through.
	case "delete":
		if req.ConfirmPath == "" {
			writeJSON(w, http.StatusBadRequest, errorResponse{
				Code:    "confirmation_required",
				Message: "delete requires confirmPath in body",
			})
			return
		}
		expandedConfirm := files.ExpandHome(req.ConfirmPath, h.Home)
		matched := false
		for _, p := range req.Paths {
			if files.ExpandHome(p, h.Home) == expandedConfirm {
				matched = true
				break
			}
		}
		if !matched {
			writeJSON(w, http.StatusBadRequest, errorResponse{
				Code:    "confirmation_required",
				Message: "confirmPath must match one of the project paths",
			})
			return
		}
	default:
		writeJSON(w, http.StatusBadRequest, errorResponse{
			Code:    "invalid_query",
			Message: "op must be 'archive' or 'delete'",
		})
		return
	}

	// Sanitize every path (no traversal, must be non-empty, must be
	// allow-listed when FileAccess is wired). Build the working list
	// of expanded absolute paths; bail at the first traversal hit
	// (that's a 400, not a per-path error).
	type pathJob struct {
		original string
		resolved string
	}
	jobs := make([]pathJob, 0, len(req.Paths))
	for _, p := range req.Paths {
		// Validate the RAW input — filepath.Clean collapses `..`
		// segments, so checking the cleaned path misses traversal
		// attempts like "/tmp/../escape" (which Clean turns into
		// "/escape"). We want to reject the request before any
		// expansion happens.
		if p == "" || hasDotDotSegment(p) {
			writeJSON(w, http.StatusBadRequest, errorResponse{
				Code:    "invalid_query",
				Message: "path contains '..' or is empty: " + p,
			})
			return
		}
		expanded := files.ExpandHome(p, h.Home)
		if expanded == "" || hasDotDotSegment(expanded) {
			writeJSON(w, http.StatusBadRequest, errorResponse{
				Code:    "invalid_query",
				Message: "path contains '..' or is empty: " + p,
			})
			return
		}
		if h.FileAccess != nil && !h.FileAccess.IsAllowed(expanded) {
			writeJSON(w, http.StatusForbidden, errorResponse{
				Code:    "path_outside_allowlist",
				Message: expanded,
			})
			return
		}
		jobs = append(jobs, pathJob{original: p, resolved: expanded})
	}

	resp := bulkProjectsResponse{Errors: []bulkProjectError{}}
	switch req.Op {
	case "archive":
		for _, j := range jobs {
			if err := h.Registry.Hide(j.resolved, true); err != nil {
				resp.Errors = append(resp.Errors, bulkProjectError{
					Path:    j.original,
					Code:    "archive_failed",
					Message: err.Error(),
				})
				continue
			}
			resp.Archived++
		}
	case "delete":
		for _, j := range jobs {
			if err := os.RemoveAll(j.resolved); err != nil {
				resp.Errors = append(resp.Errors, bulkProjectError{
					Path:    j.original,
					Code:    "delete_failed",
					Message: err.Error(),
				})
				continue
			}
			if err := h.Registry.Hide(j.resolved, true); err != nil {
				// os.RemoveAll already wiped the directory; the
				// registry hide is a best-effort cleanup so a stale
				// entry doesn't keep showing up. We still count the
				// path as deleted.
				resp.Errors = append(resp.Errors, bulkProjectError{
					Path:    j.original,
					Code:    "hide_failed",
					Message: err.Error(),
				})
				continue
			}
			resp.Deleted++
		}
	}
	if len(resp.Errors) == 0 {
		resp.Errors = nil
	}
	writeJSON(w, http.StatusOK, resp)
}

// hasDotDotSegment reports whether any path segment equals "..".
// Used by BulkHandler to reject traversal attempts before the path
// is expanded or cleaned (Clean collapses ".." so checking the
// cleaned path would miss the attack).
func hasDotDotSegment(p string) bool {
	for _, seg := range strings.Split(p, string(filepath.Separator)) {
		if seg == ".." {
			return true
		}
	}
	return false
}
