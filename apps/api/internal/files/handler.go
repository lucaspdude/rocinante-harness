package files

// HTTP handler for /api/v1/files. Read-only, gated by FileAccess.
//
// Routes:
//   GET   /api/v1/files?root=...&path=...           -> dir listing
//   GET   /api/v1/files/content?root=...&path=...  -> file body
//   PATCH /api/v1/files/content?root=...&path=...  -> replace file body
//   GET   /api/v1/git/repos?cwd=...                -> git repo BFS
//   GET   /api/v1/git/status?cwd=...               -> porcelain status
//   GET   /api/v1/cwd/browse?path=...              -> DirectoryPicker

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

// FilesHandler is mounted at /api/v1/files/*.
type FilesHandler struct {
	Access *FileAccess
	// Home is the value ExpandHome resolves "~" against. Captured
	// at construction so /api/v1/cwd/browse?path=~ resolves
	// consistently regardless of who calls the api.
	Home string
}

// NewFilesHandler creates a FilesHandler bound to the access list.
func NewFilesHandler(access *FileAccess, home string) *FilesHandler {
	return &FilesHandler{Access: access, Home: home}
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

// WriteHandler is PATCH /api/v1/files/content?root=...&path=... — replaces
// the file's contents with the JSON body. Validates allow-list, refuses
// binary files (NUL in first 1 KiB), caps at 1 MiB, writes atomically.
func (h *FilesHandler) WriteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPatch {
		writeErr(w, http.StatusMethodNotAllowed, "method_not_allowed", "use PATCH")
		return
	}
	q := r.URL.Query()
	root := q.Get("root")
	if root == "" {
		writeErr(w, http.StatusBadRequest, "invalid_path", "root required")
		return
	}
	rel := q.Get("path")
	if rel == "" {
		writeErr(w, http.StatusBadRequest, "invalid_path", "path required")
		return
	}
	if hasParentTraversal(rel) {
		writeErr(w, http.StatusBadRequest, "invalid_path", "path must not contain '..' segments")
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
	if !info.Mode().IsRegular() {
		writeErr(w, http.StatusBadRequest, "not_a_regular_file", "path is not a regular file")
		return
	}
	// Detect binary by reading the first 1 KiB of the existing file.
	if isBinary, err := isBinaryFile(target); err != nil {
		writeErr(w, http.StatusInternalServerError, "read_failed", err.Error())
		return
	} else if isBinary {
		writeErr(w, http.StatusConflict, "not_a_text_file", "existing file is binary (NUL in first 1 KiB)")
		return
	}
	if r.ContentLength > maxFileBytes {
		writeErr(w, http.StatusRequestEntityTooLarge, "file_too_large", "request body exceeds 1 MiB")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxFileBytes+1)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var mb *http.MaxBytesError
		if errors.As(err, &mb) {
			writeErr(w, http.StatusRequestEntityTooLarge, "file_too_large", "request body exceeds 1 MiB")
			return
		}
		writeErr(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	if int64(len(body)) > maxFileBytes {
		writeErr(w, http.StatusRequestEntityTooLarge, "file_too_large", "request body exceeds 1 MiB")
		return
	}
	var payload struct {
		Content string `json:"content"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	dir := filepath.Dir(target)
	tmp, err := os.CreateTemp(dir, ".tmp-write-*")
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "tmp_open_failed", err.Error())
		return
	}
	tmpName := tmp.Name()
	if _, err := tmp.WriteString(payload.Content); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		writeErr(w, http.StatusInternalServerError, "write_failed", err.Error())
		return
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		writeErr(w, http.StatusInternalServerError, "write_failed", err.Error())
		return
	}
	if err := os.Chmod(tmpName, info.Mode().Perm()); err != nil {
		os.Remove(tmpName)
		writeErr(w, http.StatusInternalServerError, "chmod_failed", err.Error())
		return
	}
	if err := os.Rename(tmpName, target); err != nil {
		os.Remove(tmpName)
		writeErr(w, http.StatusInternalServerError, "rename_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"path": target, "size": len(payload.Content)})
}

// hasParentTraversal reports whether rel contains a '..' segment
// (e.g. "../foo", "a/../b", "a/..").
func hasParentTraversal(rel string) bool {
	for _, seg := range strings.Split(rel, string(filepath.Separator)) {
		if seg == ".." {
			return true
		}
	}
	return false
}

// isBinaryFile peeks at the first 1 KiB of p and reports whether
// the file looks binary (NUL byte present). It is a heuristic; the
// caller is the WriteHandler that uses it to refuse to overwrite
// binary files with text-editor output.
func isBinaryFile(p string) (bool, error) {
	f, err := os.Open(p)
	if err != nil {
		return false, err
	}
	defer f.Close()
	head := make([]byte, 1024)
	n, err := f.Read(head)
	if err != nil && err != io.EOF {
		return false, err
	}
	return containsNUL(head[:n]), nil
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

// blockedTopLevel lists the top-level Linux pseudo-filesystems
// that the DirectoryPicker refuses to walk. On Linux these
// contain infinite or special entries that hang the listing;
// on macOS / Windows we leave them unfiltered so the local user
// can navigate their own filesystem normally.
var blockedTopLevel = map[string]bool{
	"/proc": true,
	"/sys":  true,
	"/dev":  true,
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
	path = ExpandHome(path, h.Home)
	if path == "" {
		writeErr(w, http.StatusBadRequest, "bad_request", "path required")
		return
	}
	if runtime.GOOS == "linux" && blockedTopLevel[path] {
		writeErr(w, http.StatusForbidden, "blocked_path", "path not allowed")
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

// SearchRequest is the wire shape of POST /api/v1/search.
type SearchRequest struct {
	Root    string         `json:"root"`
	Pattern string         `json:"pattern"`
	Options SearchOptions  `json:"options"`
}

// SearchOptions captures the optional flags the panel exposes.
type SearchOptions struct {
	Regex         bool   `json:"regex"`
	MaxResults    int    `json:"maxResults"`
	CaseSensitive bool   `json:"caseSensitive"`
	FileGlob      string `json:"fileGlob"`
}

// SearchMatch is one row of a ripgrep match.
type SearchMatch struct {
	Path   string `json:"path"`
	Line   int    `json:"line"`
	Column int    `json:"column"`
	Match  string `json:"match"`
}

// SearchResponse is the wire shape returned by POST /api/v1/search.
// Partial=true when ripgrep was killed by the 5s timeout; the
// caller surfaces search.partial to the user.
type SearchResponse struct {
	Results []SearchMatch `json:"results"`
	Partial bool          `json:"partial"`
}

// defaultMaxSearchResults caps how many matches a single call
// returns; the web UI requests 200 by default but a malicious /
// buggy caller could ask for more. Cap at 1000.
const defaultMaxSearchResults = 1000

// SearchHandler is POST /api/v1/search. Body: {root, pattern,
// options:{regex,maxResults,caseSensitive,fileGlob}}. Spawns
// `rg --json` with a 5s timeout, parses line-by-line, caps at
// options.maxResults (or 200 default, 1000 hard cap). On
// ripgrep-not-installed returns 503 {code:"ripgrep_not_installed"};
// on timeout returns 200 with {partial:true, results:[...so far]}.
func (h *FilesHandler) SearchHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method_not_allowed", "use POST")
		return
	}
	var req SearchRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_query", "malformed JSON body")
		return
	}
	if req.Root == "" {
		writeErr(w, http.StatusBadRequest, "invalid_query", "root required")
		return
	}
	if req.Pattern == "" {
		writeErr(w, http.StatusBadRequest, "invalid_query", "pattern required")
		return
	}
	root := req.Root
	target, err := Resolve(root, ".")
	if err != nil {
		writeErr(w, http.StatusForbidden, "path_outside_allowlist", err.Error())
		return
	}
	if !h.Access.IsAllowed(target) {
		writeErr(w, http.StatusForbidden, "path_outside_allowlist", "root not allowed")
		return
	}
	maxResults := req.Options.MaxResults
	if maxResults <= 0 {
		maxResults = 200
	}
	if maxResults > defaultMaxSearchResults {
		maxResults = defaultMaxSearchResults
	}
	matches, partial, searchErr := runRipgrep(target, req.Pattern, req.Options, maxResults)
	if searchErr != nil {
		if errors.Is(searchErr, errRipgrepMissing) {
			writeErr(w, http.StatusServiceUnavailable, "ripgrep_not_installed", "rg binary not found on PATH")
			return
		}
		writeErr(w, http.StatusInternalServerError, "search_failed", searchErr.Error())
		return
	}
	writeJSON(w, http.StatusOK, SearchResponse{Results: matches, Partial: partial})
}

// errRipgrepMissing is the sentinel returned by runRipgrep when the
// `rg` binary is not on PATH.
var errRipgrepMissing = errors.New("ripgrep_not_installed")

// runRipgrep spawns `rg --json [flags] -e PATTERN ROOT`, parses
// one JSON object per stdout line, caps the result slice at
// maxResults, and returns partial=true when the 5s context
// deadline was hit. errors.Is(err, errRipgrepMissing) when `rg` is
// absent on PATH (covers *exec.Error + fs.ErrNotExist).
func runRipgrep(root, pattern string, opts SearchOptions, maxResults int) ([]SearchMatch, bool, error) {
	ctx, cancel := newSearchTimeout()
	defer cancel()
	args := []string{"--json", "--no-config", "--no-heading", "--no-messages"}
	if opts.Regex {
		args = append(args, "--regexp")
	} else {
		args = append(args, "--fixed-strings")
	}
	if opts.CaseSensitive {
		args = append(args, "--case-sensitive")
	} else {
		args = append(args, "-i")
	}
	if opts.FileGlob != "" {
		args = append(args, "--glob", opts.FileGlob)
	}
	args = append(args, "-e", pattern, root)
	cmd := exec.CommandContext(ctx, "rg", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, false, err
	}
	if err := cmd.Start(); err != nil {
		if isMissingBinaryErr(err) {
			return nil, false, errRipgrepMissing
		}
		return nil, false, err
	}
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	out := make([]SearchMatch, 0, maxResults)
	partial := false
	for scanner.Scan() {
		line := scanner.Bytes()
		m, ok := parseRipgrepMatch(line, root)
		if !ok {
			continue
		}
		out = append(out, m)

		if len(out) >= maxResults {
			// Drain is not necessary: the cmd is killed by the
			// deadline once we return. We rely on ctx to abort
			// the goroutine still blocked on stdout.Read.
			break
		}
	}
	waitErr := cmd.Wait()
	if waitErr != nil {
		// DeadlineExceeded → partial; anything else is real.
		if ctx.Err() == context.DeadlineExceeded {
			partial = true
		} else if isMissingBinaryErr(waitErr) {
			return nil, false, errRipgrepMissing
		} else {
			// rg exits 1 when no matches were found; treat as a
			// normal empty result, not an error.
			var exitErr *exec.ExitError
			if errors.As(waitErr, &exitErr) {
				if len(out) == 0 {
					return out, false, nil
				}
				// Got some matches before rg exited non-zero;
				// still treat as a complete result.
				return out, false, nil
			}
			return nil, false, waitErr
		}
	}
	return out, partial, nil
}

// parseRipgrepMatch decodes one `rg --json` match line and
// projects it to the wire shape. Returns ok=false for non-match
// events (begin/end/summary).
func parseRipgrepMatch(line []byte, root string) (SearchMatch, bool) {
	var raw struct {
		Type string `json:"type"`
		Data struct {
			Path struct {
				Text string `json:"text"`
			} `json:"path"`
			LineNumber int `json:"line_number"`
			Submatches []struct {
				Match struct {
					Text string `json:"text"`
				} `json:"match"`
				Start int `json:"start"`
			} `json:"submatches"`
		} `json:"data"`
	}
	if err := json.Unmarshal(line, &raw); err != nil {
		return SearchMatch{}, false
	}
	if raw.Type != "match" {
		return SearchMatch{}, false
	}
	if raw.Data.Path.Text == "" || len(raw.Data.Submatches) == 0 {
		return SearchMatch{}, false
	}
	sub := raw.Data.Submatches[0]
	path := raw.Data.Path.Text
	// Strip the root prefix so the wire path is project-relative
	// (matches the convention of the file-listing endpoint).
	if root != "" {
		if rel, ok := strings.CutPrefix(path, root); ok {
			path = rel
		} else if rel, ok := strings.CutPrefix(path, root+"/"); ok {
			path = rel
		}
		path = filepath.ToSlash(strings.TrimPrefix(path, "/"))
	}
	return SearchMatch{
		Path:   path,
		Line:   raw.Data.LineNumber,
		Column: sub.Start + 1,
		Match:  sub.Match.Text,
	}, true
}

// isMissingBinaryErr reports whether err from exec.Command.Start
// / Wait means the binary isn't on PATH. exec.LookPath returns
// *exec.Error with Err == fs.ErrNotExist when the binary is
// absent; that surfaces as the cmd's start error.
// isMissingBinaryErr reports whether err from exec.Command.Start
// / Wait means the binary isn't on PATH. exec.LookPath surfaces
// this as *exec.Error with a string-wrapped message — not
// fs.ErrNotExist. We match on the error type only; the cmd was
// constructed with a known binary name ("rg"), so any *exec.Error
// from Start() means "binary missing".
func isMissingBinaryErr(err error) bool {
	if err == nil {
		return false
	}
	var execErr *exec.Error
	return errors.As(err, &execErr)
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
