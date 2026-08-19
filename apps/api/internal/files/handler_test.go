package files

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
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
