package files

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)
// newTestBrowseHandler wires a FilesHandler pointed at a temp dir
// that doubles as the user's home dir. The same temp dir is
// allow-listed so ExpandHome("~") resolves to it and IsAllowed
// returns true for the path it yields.
func newTestBrowseHandler(t *testing.T) (*FilesHandler, string) {
	t.Helper()
	home := t.TempDir()
	// Add a sub-dir we can browse into.
	if err := os.MkdirAll(filepath.Join(home, "projects"), 0o755); err != nil {
		t.Fatal(err)
	}
	fa := NewFileAccess()
	if err := fa.Allow(home); err != nil {
		t.Fatal(err)
	}
	return NewFilesHandler(fa, home), home
}

func getJSON(t *testing.T, h http.HandlerFunc, path string) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	var body map[string]any
	if rr.Body.Len() > 0 {
		if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
	}
	return rr.Code, body
}

func TestBrowseHandlerExpandsTilde(t *testing.T) {
	h, _ := newTestBrowseHandler(t)
	code, body := getJSON(t, h.BrowseHandler, "/api/v1/cwd/browse?path=~")
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%v", code, body)
	}
	// EvalSymlinks on a tmp dir resolves to /private/var/... on
	// macOS. We just check the response is non-empty and contains
	// the sub-dir we created.
	if body["path"] == nil || body["path"] == "" {
		t.Errorf("expected path to be set; got %v", body["path"])
	}
	entries, ok := body["entries"].([]any)
	if !ok {
		t.Fatalf("entries is not an array: %T", body["entries"])
	}
	hasProjects := false
	for _, e := range entries {
		m := e.(map[string]any)
		if m["name"] == "projects" && m["is_dir"] == true {
			hasProjects = true
		}
	}
	if !hasProjects {
		t.Errorf("expected to find 'projects' entry; got %v", entries)
	}
}

func TestBrowseHandlerAcceptsTildeSubpath(t *testing.T) {
	h, _ := newTestBrowseHandler(t)
	code, body := getJSON(t, h.BrowseHandler, "/api/v1/cwd/browse?path=~/projects")
	if code != http.StatusOK {
		t.Fatalf("status = %d; body=%v", code, body)
	}
	if body["path"] == nil || body["path"] == "" {
		t.Errorf("expected path to be set; got %v", body["path"])
	}
}

func TestBrowseHandlerRejectsBlockedPath(t *testing.T) {
	h, _ := newTestBrowseHandler(t)
	// /proc is in blockedTopLevel; must return 403 with the
	// blocked_path code regardless of OS (handler short-circuits
	// on linux; on macOS /proc is not blocked but also not
	// allow-listed so we accept either 403 from blocked or 403
	// from allow-list — both are forbidden).
	code, _ := getJSON(t, h.BrowseHandler, "/api/v1/cwd/browse?path=/proc")
	if code != http.StatusForbidden {
		t.Errorf("/proc: status = %d, want 403", code)
	}
}

func TestBrowseHandlerRequiresPath(t *testing.T) {
	h, _ := newTestBrowseHandler(t)
	code, _ := getJSON(t, h.BrowseHandler, "/api/v1/cwd/browse")
	if code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", code)
	}
}

// patchJSON issues a PATCH to the given handler with the given
// JSON body and returns the status code + decoded body.
func patchJSON(t *testing.T, h http.HandlerFunc, path string, body []byte) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPatch, path, nil)
	if body != nil {
		req.Body = io.NopCloser(bytes.NewReader(body))
		req.ContentLength = int64(len(body))
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	var out map[string]any
	if rr.Body.Len() > 0 {
		if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
			t.Fatalf("decode body: %v", err)
		}
	}
	return rr.Code, out
}

func TestWriteHandlerHappyPath(t *testing.T) {
	h, home := newTestBrowseHandler(t)
	target := filepath.Join(home, "notes.txt")
	if err := os.WriteFile(target, []byte("hello, world"), 0o644); err != nil {
		t.Fatal(err)
	}
	payload, _ := json.Marshal(map[string]string{"content": "new content\n"})
	root := url.QueryEscape(home)
	code, body := patchJSON(t, h.WriteHandler, "/api/v1/files/content?root="+root+"&path=notes.txt", payload)
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%v", code, body)
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new content\n" {
		t.Errorf("file content = %q, want %q", got, "new content\n")
	}
	// Re-read via ContentHandler with a raw GET (ContentHandler
	// returns plain text, not JSON).
	url := "/api/v1/files/content?root=" + url.QueryEscape(home) + "&path=notes.txt"
	req := httptest.NewRequest(http.MethodGet, url, nil)
	rr := httptest.NewRecorder()
	h.ContentHandler(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("reread status = %d, want 200; body=%q", rr.Code, rr.Body.String())
	}
	if rr.Body.String() != "new content\n" {
		t.Errorf("reread content = %q, want %q", rr.Body.String(), "new content\n")
	}
}

func TestWriteHandlerInvalidPath(t *testing.T) {
	h, home := newTestBrowseHandler(t)
	root := url.QueryEscape(home)
	cases := []struct {
		name  string
		query string
	}{
		{"empty root", "/api/v1/files/content?path=notes.txt"},
		{"empty path", "/api/v1/files/content?root=" + root},
		{"dotdot in path", "/api/v1/files/content?root=" + root + "&path=../escape.txt"},
		{"dotdot nested", "/api/v1/files/content?root=" + root + "&path=a/../escape.txt"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			payload, _ := json.Marshal(map[string]string{"content": "x"})
			code, body := patchJSON(t, h.WriteHandler, tc.query, payload)
			if code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400; body=%v", code, body)
			}
			if body["code"] != "invalid_path" {
				t.Errorf("code = %v, want invalid_path", body["code"])
			}
		})
	}
}

func TestWriteHandlerOutsideAllowList(t *testing.T) {
	h, _ := newTestBrowseHandler(t)
	// A root that is NOT in the allow-list.
	other := t.TempDir()
	if err := os.WriteFile(filepath.Join(other, "secret.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	root := url.QueryEscape(other)
	payload, _ := json.Marshal(map[string]string{"content": "x"})
	code, body := patchJSON(t, h.WriteHandler, "/api/v1/files/content?root="+root+"&path=secret.txt", payload)
	if code != http.StatusForbidden {
		t.Errorf("status = %d, want 403; body=%v", code, body)
	}
	if body["code"] != "path_outside_allowlist" {
		t.Errorf("code = %v, want path_outside_allowlist", body["code"])
	}
}

func TestWriteHandlerBinaryFile(t *testing.T) {
	h, home := newTestBrowseHandler(t)
	target := filepath.Join(home, "blob.bin")
	// Pre-existing file with a NUL byte in the first 1 KiB.
	if err := os.WriteFile(target, []byte("before\x00after"), 0o644); err != nil {
		t.Fatal(err)
	}
	root := url.QueryEscape(home)
	payload, _ := json.Marshal(map[string]string{"content": "new content"})
	code, body := patchJSON(t, h.WriteHandler, "/api/v1/files/content?root="+root+"&path=blob.bin", payload)
	if code != http.StatusConflict {
		t.Errorf("status = %d, want 409; body=%v", code, body)
	}
	if body["code"] != "not_a_text_file" {
		t.Errorf("code = %v, want not_a_text_file", body["code"])
	}
	// File must not be modified.
	got, _ := os.ReadFile(target)
	if string(got) != "before\x00after" {
		t.Errorf("file was modified: %q", got)
	}
}

func TestWriteHandlerTooLarge(t *testing.T) {
	h, home := newTestBrowseHandler(t)
	target := filepath.Join(home, "big.txt")
	if err := os.WriteFile(target, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	root := url.QueryEscape(home)
	// 1 MiB + 1 byte body.
	body := bytes.Repeat([]byte("a"), (1<<20)+1)
	payload, _ := json.Marshal(map[string]string{"content": string(body)})
	code, body2 := patchJSON(t, h.WriteHandler, "/api/v1/files/content?root="+root+"&path=big.txt", payload)
	if code != http.StatusRequestEntityTooLarge {
		t.Errorf("status = %d, want 413; body=%v", code, body2)
	}
	if body2["code"] != "file_too_large" {
		t.Errorf("code = %v, want file_too_large", body2["code"])
	}
}

// postJSON issues a POST to the given handler with the given
// JSON body and returns the status code + decoded body.
func postJSON(t *testing.T, h http.HandlerFunc, path string, body []byte) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, nil)
	if body != nil {
		req.Body = io.NopCloser(bytes.NewReader(body))
		req.ContentLength = int64(len(body))
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	var out map[string]any
	if rr.Body.Len() > 0 {
		if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
			t.Fatalf("decode body: %v", err)
		}
	}
	return rr.Code, out
}

// requireRipgrep skips the test if `rg` is not on PATH. CI hosts
// ship ripgrep (used by ssh/test.go and the SSH PR); local macOS
// installs usually have it via brew.
func requireRipgrep(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("rg"); err != nil {
		t.Skip("rg not on PATH; skipping ripgrep-backed search test")
	}
}

// writeSearchProject seeds a tiny project tree with known
// strings the SearchHandler can find.
func writeSearchProject(t *testing.T, home string) {
	t.Helper()
	files := map[string]string{
		"src/main.go":    "package main\n// TODO: refactor\nfunc main() {}\n",
		"src/util.go":    "package main\nfunc helper() { /* TODO note */ }\n",
		"README.md":      "TODO: write docs\n",
		"docs/notes.txt": "no markers here\n",
	}
	for rel, content := range files {
		full := filepath.Join(home, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestSearchHandlerHappyPath(t *testing.T) {
	requireRipgrep(t)
	h, home := newTestBrowseHandler(t)
	writeSearchProject(t, home)
	payload, _ := json.Marshal(SearchRequest{
		Root:    home,
		Pattern: "TODO",
		Options: SearchOptions{MaxResults: 50},
	})
	code, body := postJSON(t, h.SearchHandler, "/api/v1/search", payload)
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%v", code, body)
	}
	results, ok := body["results"].([]any)
	if !ok {
		t.Fatalf("results is not an array: %T", body["results"])
	}
	if len(results) < 2 {
		t.Fatalf("expected at least 2 TODO matches; got %d: %v", len(results), results)
	}
	if partial, _ := body["partial"].(bool); partial {
		t.Errorf("partial should be false on a small project; got true")
	}
	// First match should reference one of the TODO-bearing files.
	first := results[0].(map[string]any)
	if path, _ := first["path"].(string); path == "" {
		t.Errorf("first match has no path: %v", first)
	}
	if line, _ := first["line"].(float64); line <= 0 {
		t.Errorf("first match has invalid line: %v", first)
	}
	if match, _ := first["match"].(string); match != "TODO" {
		t.Errorf("first match.match = %q, want TODO", match)
	}
}

func TestSearchHandlerRequiresRootAndPattern(t *testing.T) {
	requireRipgrep(t)
	h, _ := newTestBrowseHandler(t)
	cases := []struct {
		name string
		req  SearchRequest
	}{
		{"empty root", SearchRequest{Pattern: "TODO"}},
		{"empty pattern", SearchRequest{Root: "/tmp"}},
		{"both empty", SearchRequest{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			payload, _ := json.Marshal(tc.req)
			code, body := postJSON(t, h.SearchHandler, "/api/v1/search", payload)
			if code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400; body=%v", code, body)
			}
			if body["code"] != "invalid_query" {
				t.Errorf("code = %v, want invalid_query", body["code"])
			}
		})
	}
}

func TestSearchHandlerOutsideAllowList(t *testing.T) {
	requireRipgrep(t)
	h, _ := newTestBrowseHandler(t)
	other := t.TempDir()
	if err := os.WriteFile(filepath.Join(other, "secret.go"), []byte("TODO: hide"), 0o644); err != nil {
		t.Fatal(err)
	}
	payload, _ := json.Marshal(SearchRequest{Root: other, Pattern: "TODO"})
	code, body := postJSON(t, h.SearchHandler, "/api/v1/search", payload)
	if code != http.StatusForbidden {
		t.Errorf("status = %d, want 403; body=%v", code, body)
	}
	if body["code"] != "path_outside_allowlist" {
		t.Errorf("code = %v, want path_outside_allowlist", body["code"])
	}
}

func TestSearchHandlerMaxResultsCap(t *testing.T) {
	requireRipgrep(t)
	h, home := newTestBrowseHandler(t)
	// Generate 5 files each with 3 matches = 15 total. Cap at 4.
	for i := range 5 {
		path := filepath.Join(home, fmt.Sprintf("f%d.txt", i))
		if err := os.WriteFile(path, []byte("NEEDLE\nNEEDLE\nNEEDLE\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	payload, _ := json.Marshal(SearchRequest{
		Root:    home,
		Pattern: "NEEDLE",
		Options: SearchOptions{MaxResults: 4},
	})
	code, body := postJSON(t, h.SearchHandler, "/api/v1/search", payload)
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%v", code, body)
	}
	results, _ := body["results"].([]any)
	if len(results) != 4 {
		t.Errorf("results count = %d, want 4 (cap honoured)", len(results))
	}
}

func TestSearchHandlerRegexMode(t *testing.T) {
	requireRipgrep(t)
	h, home := newTestBrowseHandler(t)
	if err := os.WriteFile(filepath.Join(home, "a.txt"), []byte("foo(bar)\nbaz\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Pattern uses parens → only matches in regex mode (literal
	// mode would search for the literal "()" which appears nowhere).
	payload, _ := json.Marshal(SearchRequest{
		Root:    home,
		Pattern: "foo\\(bar\\)",
		Options: SearchOptions{Regex: true},
	})
	code, body := postJSON(t, h.SearchHandler, "/api/v1/search", payload)
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%v", code, body)
	}
	results, _ := body["results"].([]any)
	if len(results) != 1 {
		t.Fatalf("regex matches = %d, want 1; body=%v", len(results), body)
	}
	first := results[0].(map[string]any)
	if got, _ := first["path"].(string); got != "a.txt" {
		t.Errorf("path = %q, want a.txt", got)
	}
}

func TestSearchHandlerMethodNotAllowed(t *testing.T) {
	h, _ := newTestBrowseHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/search", nil)
	rr := httptest.NewRecorder()
	h.SearchHandler(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", rr.Code)
	}
}

func TestSearchHandlerRipgrepMissing(t *testing.T) {
	// Stub exec.LookPath to report rg as missing. The handler's
	// ripgrep-not-installed branch must surface 503.
	t.Setenv("PATH", t.TempDir()) // empty PATH → rg cannot be found
	h, home := newTestBrowseHandler(t)
	writeSearchProject(t, home)
	payload, _ := json.Marshal(SearchRequest{Root: home, Pattern: "TODO"})
	code, body := postJSON(t, h.SearchHandler, "/api/v1/search", payload)
	if code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503; body=%v", code, body)
	}
	if body["code"] != "ripgrep_not_installed" {
		t.Errorf("code = %v, want ripgrep_not_installed", body["code"])
	}
}
