package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lucaspdude/rocinante-harness/apps/api/internal/projects"
)

// newBulkTestHandler wires a ProjectsHandlers with a fresh
// registry rooted at t.TempDir() and a nil FileAccess so the
// per-path allow-list check is skipped (FileAccess is exercised by
// files/access_test.go). Returns a router that mounts just
// POST /api/v1/projects/bulk.
func newBulkTestHandler(t *testing.T) (*ProjectsHandlers, http.Handler) {
	t.Helper()
	dir := t.TempDir()
	reg := projects.NewRegistry(dir)
	h := &ProjectsHandlers{
		Registry:   reg,
		FileAccess: nil,
		Home:       "/root",
	}
	r := http.NewServeMux()
	r.HandleFunc("/api/v1/projects/bulk", h.BulkHandler)
	return h, r
}

func doBulk(t *testing.T, h http.Handler, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/bulk", bytes.NewBufferString(body))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

func decodeBulk(t *testing.T, rr *httptest.ResponseRecorder) bulkProjectsResponse {
	t.Helper()
	var out bulkProjectsResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode body: %v (raw=%s)", err, rr.Body.String())
	}
	return out
}

// Seed a small registry and confirm POST /bulk {op:"archive"} hides
// every listed project. The two extra entries that are NOT in the
// request stay visible.
func TestBulkHandlerArchiveThree(t *testing.T) {
	h, r := newBulkTestHandler(t)
	for _, p := range []string{"/tmp/a", "/tmp/b", "/tmp/c", "/tmp/d"} {
		if _, err := h.Registry.Upsert(p, filepath.Base(p), "", false); err != nil {
			t.Fatalf("seed %s: %v", p, err)
		}
	}
	body := `{"op":"archive","paths":["/tmp/a","/tmp/b","/tmp/c"]}`
	rr := doBulk(t, r, body)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	out := decodeBulk(t, rr)
	if out.Archived != 3 {
		t.Errorf("archived = %d, want 3; errors=%+v", out.Archived, out.Errors)
	}
	if len(out.Errors) != 0 {
		t.Errorf("unexpected errors: %+v", out.Errors)
	}
	for _, p := range []string{"/tmp/a", "/tmp/b", "/tmp/c"} {
		got, ok := h.Registry.Get(p)
		if !ok || !got.Hidden {
			t.Errorf("%s not hidden: ok=%v got=%+v", p, ok, got)
		}
	}
	got, ok := h.Registry.Get("/tmp/d")
	if !ok || got.Hidden {
		t.Errorf("/tmp/d should stay visible: ok=%v hidden=%v", ok, got.Hidden)
	}
}

// Seed a real on-disk directory, register it, and confirm
// POST /bulk {op:"delete", confirmPath matching} wipes the directory
// and hides the registry entry.
func TestBulkHandlerDeleteWithConfirm(t *testing.T) {
	h, r := newBulkTestHandler(t)
	root := t.TempDir()
	target := filepath.Join(root, "delete-me")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(target, "a.txt"), []byte("hi"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := h.Registry.Upsert(target, "delete-me", "", false); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	body, _ := json.Marshal(map[string]any{
		"op":          "delete",
		"paths":       []string{target},
		"confirmPath": target,
	})
	rr := doBulk(t, r, string(body))
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	out := decodeBulk(t, rr)
	if out.Deleted != 1 {
		t.Errorf("deleted = %d, want 1; errors=%+v", out.Deleted, out.Errors)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Errorf("directory still on disk: stat err=%v", err)
	}
	got, ok := h.Registry.Get(target)
	if !ok || !got.Hidden {
		t.Errorf("registry not hidden: ok=%v got=%+v", ok, got)
	}
}

func TestBulkHandlerDeleteMissingConfirmPath(t *testing.T) {
	h, r := newBulkTestHandler(t)
	if _, err := h.Registry.Upsert("/tmp/x", "X", "", false); err != nil {
		t.Fatalf("seed: %v", err)
	}
	body := `{"op":"delete","paths":["/tmp/x"]}`
	rr := doBulk(t, r, body)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rr.Code, rr.Body.String())
	}
	var e errorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &e); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if e.Code != "confirmation_required" {
		t.Errorf("code = %q, want confirmation_required", e.Code)
	}
	if got, ok := h.Registry.Get("/tmp/x"); !ok || got.Hidden {
		t.Errorf("registry should stay visible: ok=%v hidden=%v", ok, got.Hidden)
	}
}

func TestBulkHandlerDeleteMismatchedConfirm(t *testing.T) {
	h, r := newBulkTestHandler(t)
	if _, err := h.Registry.Upsert("/tmp/x", "X", "", false); err != nil {
		t.Fatalf("seed: %v", err)
	}
	body := `{"op":"delete","paths":["/tmp/x"],"confirmPath":"/tmp/other"}`
	rr := doBulk(t, r, body)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rr.Code, rr.Body.String())
	}
	var e errorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &e); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if e.Code != "confirmation_required" {
		t.Errorf("code = %q, want confirmation_required", e.Code)
	}
}

func TestBulkHandlerEmptyPaths(t *testing.T) {
	_, r := newBulkTestHandler(t)
	body := `{"op":"archive","paths":[]}`
	rr := doBulk(t, r, body)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rr.Code, rr.Body.String())
	}
	var e errorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &e); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if e.Code != "invalid_query" {
		t.Errorf("code = %q, want invalid_query", e.Code)
	}
}

func TestBulkHandlerUnknownOp(t *testing.T) {
	h, r := newBulkTestHandler(t)
	if _, err := h.Registry.Upsert("/tmp/x", "X", "", false); err != nil {
		t.Fatalf("seed: %v", err)
	}
	body := `{"op":"nuke","paths":["/tmp/x"]}`
	rr := doBulk(t, r, body)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
	var e errorResponse
	_ = json.Unmarshal(rr.Body.Bytes(), &e)
	if e.Code != "invalid_query" {
		t.Errorf("code = %q, want invalid_query", e.Code)
	}
}

func TestBulkHandlerRejectsTraversal(t *testing.T) {
	h, r := newBulkTestHandler(t)
	if _, err := h.Registry.Upsert("/tmp/legit", "Legit", "", false); err != nil {
		t.Fatalf("seed: %v", err)
	}
	body := `{"op":"archive","paths":["/tmp/../escape"]}`
	rr := doBulk(t, r, body)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
	if got, ok := h.Registry.Get("/tmp/legit"); !ok || got.Hidden {
		t.Errorf("traversal request must not archive the legit entry: ok=%v hidden=%v", ok, got.Hidden)
	}
	var e errorResponse
	_ = json.Unmarshal(rr.Body.Bytes(), &e)
	if e.Code != "invalid_query" {
		t.Errorf("code = %q, want invalid_query", e.Code)
	}
}

func TestBulkHandlerMethodNotAllowed(t *testing.T) {
	_, r := newBulkTestHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/bulk", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", rr.Code)
	}
	if got := rr.Header().Get("Allow"); !strings.Contains(got, "POST") {
		t.Errorf("Allow header = %q, want it to include POST", got)
	}
}
