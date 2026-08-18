package files

// HTTP handler for /api/v1/files. Read-only, gated by FileAccess.
//
// Routes:
//   GET /api/v1/files?root=...&path=...              -> dir listing
//   GET /api/v1/files/content?root=...&path=...     -> file body
//   GET /api/v1/git/repos?cwd=...                   -> git repo BFS
//   GET /api/v1/git/status?cwd=...                  -> porcelain status
//   GET /api/v1/cwd/browse?path=...                 -> DirectoryPicker

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// FilesHandler is mounted at /api/v1/files/*.
type FilesHandler struct {
	Access *FileAccess
}

// NewFilesHandler creates a FilesHandler bound to the access list.
func NewFilesHandler(access *FileAccess) *FilesHandler {
	return &FilesHandler{Access: access}
}

// Entry is one row of a directory listing.
type Entry struct {
	Name    string `json:"name"`
	IsDir   bool   `json:"is_dir"`
	Size    int64  `json:"size"`
	Mode    string `json:"mode"`
	ModTime string `json:"mod_time"`
}

// ListResponse is the wire shape of GET /files.
type ListResponse struct {
	Root    string  `json:"root"`
	Path    string  `json:"path"`
	Entries []Entry `json:"entries"`
}

const maxFileBytes = 1 << 20 // 1 MiB

// ListHandler is GET /api/v1/files?root=...&path=...
func (h *FilesHandler) ListHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method_not_allowed", "use GET")
		return
	}
	q := r.URL.Query()
	root := q.Get("root")
	if root == "" {
		writeErr(w, http.StatusBadRequest, "bad_request", "root required")
		return
	}
	rel := q.Get("path")
	if rel == "" {
		rel = "."
	}
	target, err := Resolve(root, rel)
	if err != nil {
		writeErr(w, http.StatusForbidden, "path_outside_allowlist", err.Error())
		return
	}
	if !h.Access.IsAllowed(target) {
		writeErr(w, http.StatusForbidden, "path_outside_allowlist", "root not allowed")
		return
	}
	dirEntries, err := os.ReadDir(target)
	if err != nil {
		if os.IsNotExist(err) {
			writeErr(w, http.StatusNotFound, "dir_not_found", target)
			return
		}
		writeErr(w, http.StatusInternalServerError, "readdir_failed", err.Error())
		return
	}
	out := make([]Entry, 0, len(dirEntries))
	for _, e := range dirEntries {
		info, infoErr := e.Info()
		if infoErr != nil {
			continue
		}
		out = append(out, Entry{
			Name:    e.Name(),
			IsDir:   e.IsDir(),
			Size:    info.Size(),
			Mode:    info.Mode().String(),
			ModTime: info.ModTime().UTC().Format(time.RFC3339),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].IsDir != out[j].IsDir {
			return out[i].IsDir
		}
		return out[i].Name < out[j].Name
	})
	writeJSON(w, http.StatusOK, ListResponse{
		Root:    root,
		Path:    rel,
		Entries: out,
	})
}

// BinaryMarkerResponse is sent when the file is binary or too large.
type BinaryMarkerResponse struct {
	Kind      string `json:"kind"` // "binary" | "too_large"
	Size      int64  `json:"size"`
	Truncated bool   `json:"truncated,omitempty"`
}

// ContentHandler serves raw file bytes for text files. Binary
// detection: first 512 bytes contain a NUL byte → binary.
func (h *FilesHandler) ContentHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method_not_allowed", "use GET")
		return
	}
	q := r.URL.Query()
	root := q.Get("root")
	if root == "" {
		writeErr(w, http.StatusBadRequest, "bad_request", "root required")
		return
	}
	rel := q.Get("path")
	if rel == "" {
		writeErr(w, http.StatusBadRequest, "bad_request", "path required")
		return
	}
	target, err := Resolve(root, rel)
	if err != nil {
		writeErr(w, http.StatusForbidden, "path_outside_allowlist", err.Error())
		return
	}
	if !h.Access.IsAllowed(target) {
		writeErr(w, http.StatusForbidden, "path_outside_allowlist", "root not allowed")
		return
	}
	info, err := os.Stat(target)
	if err != nil {
		if os.IsNotExist(err) {
			writeErr(w, http.StatusNotFound, "file_not_found", target)
			return
		}
		writeErr(w, http.StatusInternalServerError, "stat_failed", err.Error())
		return
	}
	if info.IsDir() {
		writeErr(w, http.StatusBadRequest, "is_dir", "path is a directory")
		return
	}
	if info.Size() > 10*maxFileBytes {
		writeErr(w, http.StatusRequestEntityTooLarge, "path_too_long", "file exceeds 10 MiB hard cap")
		return
	}
	if info.Size() > maxFileBytes {
		w.Header().Set("Content-Type", "application/json")
		writeJSON(w, http.StatusOK, BinaryMarkerResponse{
			Kind:      "too_large",
			Size:      info.Size(),
			Truncated: true,
		})
		return
	}
	f, err := os.Open(target)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "open_failed", err.Error())
		return
	}
	defer f.Close()
	head := make([]byte, 512)
	n, _ := f.Read(head)
	head = head[:n]
	if containsNUL(head) {
		w.Header().Set("Content-Type", "application/json")
		writeJSON(w, http.StatusOK, BinaryMarkerResponse{
			Kind: "binary",
			Size: info.Size(),
		})
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(head); err != nil {
		return
	}
	if _, err := io.Copy(w, f); err != nil {
		return
	}
}

func containsNUL(b []byte) bool {
	for _, c := range b {
		if c == 0 {
			return true
		}
	}
	return false
}

// BrowseResponse is the wire shape of /cwd/browse.
type BrowseResponse struct {
	Path    string  `json:"path"`
	Parent  string  `json:"parent,omitempty"`
	Entries []Entry `json:"entries"`
}

// BrowseHandler is GET /api/v1/cwd/browse?path=... — exposes
// the host filesystem via the same allow-list.
func (h *FilesHandler) BrowseHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method_not_allowed", "use GET")
		return
	}
	q := r.URL.Query()
	path := q.Get("path")
	if path == "" {
		writeErr(w, http.StatusBadRequest, "bad_request", "path required")
		return
	}
	if !h.Access.IsAllowed(path) {
		writeErr(w, http.StatusForbidden, "path_outside_allowlist", "path not allowed")
		return
	}
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			writeErr(w, http.StatusNotFound, "dir_not_found", path)
			return
		}
		writeErr(w, http.StatusInternalServerError, "stat_failed", err.Error())
		return
	}
	if !info.IsDir() {
		writeErr(w, http.StatusBadRequest, "not_a_directory", path)
		return
	}
	dirEntries, err := os.ReadDir(path)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "readdir_failed", err.Error())
		return
	}
	out := make([]Entry, 0, len(dirEntries))
	for _, e := range dirEntries {
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		i, infoErr := e.Info()
		if infoErr != nil {
			continue
		}
		out = append(out, Entry{
			Name:    e.Name(),
			IsDir:   e.IsDir(),
			Size:    i.Size(),
			Mode:    i.Mode().String(),
			ModTime: i.ModTime().UTC().Format(time.RFC3339),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].IsDir != out[j].IsDir {
			return out[i].IsDir
		}
		return out[i].Name < out[j].Name
	})
	parent := ""
	if dir := filepath.Dir(path); dir != path {
		parent = dir
	}
	writeJSON(w, http.StatusOK, BrowseResponse{
		Path:    path,
		Parent:  parent,
		Entries: out,
	})
}

// GitHandler is mounted at /api/v1/git/*.
type GitHandler struct {
	Access *FileAccess
}

// NewGitHandler creates a GitHandler bound to the access list.
func NewGitHandler(access *FileAccess) *GitHandler {
	return &GitHandler{Access: access}
}

// Repo is one git repository discovered in a BFS scan.
type Repo struct {
	Cwd     string `json:"cwd"`
	Name    string `json:"name"`
	Head    string `json:"head"`
	Branch  string `json:"branch"`
	IsDirty bool   `json:"is_dirty"`
	Path    string `json:"path"`
}

// ReposResponse is the wire shape.
type ReposResponse struct {
	Repos []Repo `json:"repos"`
}

var gitSkip = map[string]struct{}{
	"node_modules": {}, "target": {}, "dist": {}, "build": {},
	".next": {}, ".venv": {}, "__pycache__": {}, ".cache": {},
}

const maxDepth = 4

// ReposHandler is GET /api/v1/git/repos?cwd=...
func (g *GitHandler) ReposHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method_not_allowed", "use GET")
		return
	}
	cwd := r.URL.Query().Get("cwd")
	if cwd == "" {
		writeErr(w, http.StatusBadRequest, "bad_request", "cwd required")
		return
	}
	if !g.Access.IsAllowed(cwd) {
		writeErr(w, http.StatusForbidden, "path_outside_allowlist", "cwd not allowed")
		return
	}
	info, err := os.Stat(cwd)
	if err != nil {
		if os.IsNotExist(err) {
			writeErr(w, http.StatusNotFound, "dir_not_found", cwd)
			return
		}
		writeErr(w, http.StatusInternalServerError, "stat_failed", err.Error())
		return
	}
	if !info.IsDir() {
		writeErr(w, http.StatusBadRequest, "not_a_directory", cwd)
		return
	}
	repos := bfsFindRepos(cwd, 0)
	writeJSON(w, http.StatusOK, ReposResponse{Repos: repos})
}

func bfsFindRepos(root string, depth int) []Repo {
	var out []Repo
	if depth > maxDepth {
		return out
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return out
	}
	for _, e := range entries {
		name := e.Name()
		if _, skip := gitSkip[name]; skip {
			continue
		}
		if !e.IsDir() {
			continue
		}
		sub := filepath.Join(root, name)
		if name == ".git" {
			continue
		}
		gitDir := filepath.Join(sub, ".git")
		if _, err := os.Stat(gitDir); err == nil {
			repo := Repo{
				Cwd:  root,
				Name: filepath.Base(root),
				Path: sub,
			}
			repo.Head, repo.Branch, repo.IsDirty = gitStatus(sub)
			out = append(out, repo)
			continue
		}
		out = append(out, bfsFindRepos(sub, depth+1)...)
	}
	return out
}

// gitStatus shells out to git; non-fatal on failures (returns 0 values).
func gitStatus(cwd string) (head, branch string, dirty bool) {
	ctx, cancel := newProcTimeout()
	defer cancel()
	branchOut, err := exec.CommandContext(ctx, "git", "-C", cwd, "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		return "", "", false
	}
	branch = strings.TrimSpace(string(branchOut))
	headOut, err := exec.CommandContext(ctx, "git", "-C", cwd, "rev-parse", "--short", "HEAD").Output()
	if err == nil {
		head = strings.TrimSpace(string(headOut))
	}
	statusOut, err := exec.CommandContext(ctx, "git", "-C", cwd, "status", "--porcelain").Output()
	if err == nil {
		dirty = len(strings.TrimSpace(string(statusOut))) > 0
	}
	return head, branch, dirty
}

// FileStatus is one row of git status.
type FileStatus struct {
	Path   string `json:"path"`
	Status string `json:"status"`
}

type StatusResponse struct {
	Cwd   string       `json:"cwd"`
	Repo  string       `json:"repo,omitempty"`
	Files []FileStatus `json:"files"`
	Clean bool         `json:"clean"`
}

// StatusHandler is GET /api/v1/git/status?cwd=...
func (g *GitHandler) StatusHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method_not_allowed", "use GET")
		return
	}
	cwd := r.URL.Query().Get("cwd")
	if cwd == "" {
		writeErr(w, http.StatusBadRequest, "bad_request", "cwd required")
		return
	}
	if !g.Access.IsAllowed(cwd) {
		writeErr(w, http.StatusForbidden, "path_outside_allowlist", "cwd not allowed")
		return
	}
	ctx, cancel := newProcTimeout()
	defer cancel()
	out, err := exec.CommandContext(ctx, "git", "-C", cwd, "status", "--porcelain").Output()
	if err != nil {
		writeErr(w, http.StatusNotFound, "dir_not_found", cwd)
		return
	}
	files := parsePorcelain(strings.TrimSpace(string(out)))
	resp := StatusResponse{
		Cwd:   cwd,
		Repo:  filepath.Base(cwd),
		Files: files,
		Clean: len(files) == 0,
	}
	writeJSON(w, http.StatusOK, resp)
}

func parsePorcelain(s string) []FileStatus {
	if s == "" {
		return nil
	}
	var out []FileStatus
	for _, line := range strings.Split(s, "\n") {
		if len(line) < 3 || line[2] != ' ' {
			continue
		}
		status := strings.TrimSpace(line[0:2])
		path := line[3:]
		if status == "R " || status == "C " {
			parts := strings.SplitN(path, " -> ", 2)
			if len(parts) == 2 {
				path = parts[1]
			}
		}
		out = append(out, FileStatus{Path: path, Status: status})
	}
	return out
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeErr(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{"code": code, "message": message})
}
