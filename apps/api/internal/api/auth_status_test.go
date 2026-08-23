package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-chi/chi/v5"
)

// newAuthStatusRouter mounts only the public /api/v1/auth/status
// route. Mirrors newPublicMeRouter in me_test.go.
func newAuthStatusRouter(shareDir string) http.Handler {
	r := chi.NewRouter()
	r.Get("/api/v1/auth/status", AuthStatusHandler(shareDir))
	return r
}

func TestAuthStatusHandlerNotInitialized(t *testing.T) {
	dir := t.TempDir()
	r := newAuthStatusRouter(dir)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/status", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rr.Code, rr.Body.String())
	}
	var body AuthStatusResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Initialized {
		t.Errorf("initialized = true, want false (no .ed25519 on disk)")
	}
	if body.AuthRequired {
		t.Errorf("auth_required = true, want false when not initialized")
	}
	if body.DeviceKnown {
		t.Errorf("device_known = true, want false (no cookie sent)")
	}
}

func TestAuthStatusHandlerInitialized(t *testing.T) {
	dir := t.TempDir()
	// Touch the .ed25519 file so os.Stat reports it present.
	if err := os.WriteFile(filepath.Join(dir, ".ed25519"), []byte("stub"), 0o600); err != nil {
		t.Fatalf("write .ed25519: %v", err)
	}
	r := newAuthStatusRouter(dir)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/status", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rr.Code, rr.Body.String())
	}
	var body AuthStatusResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if !body.Initialized {
		t.Errorf("initialized = false, want true")
	}
	if !body.AuthRequired {
		t.Errorf("auth_required = false, want true when initialized")
	}
	if body.DeviceKnown {
		t.Errorf("device_known = true, want false (no cookie sent)")
	}
}

func TestAuthStatusHandlerDeviceKnown(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".ed25519"), []byte("stub"), 0o600); err != nil {
		t.Fatalf("write .ed25519: %v", err)
	}
	r := newAuthStatusRouter(dir)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/status", nil)
	req.AddCookie(&http.Cookie{Name: "rh-device-id", Value: "abc"})
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rr.Code, rr.Body.String())
	}
	var body AuthStatusResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if !body.DeviceKnown {
		t.Errorf("device_known = false, want true (cookie sent)")
	}
}

func TestAuthStatusHandlerNoShareDir(t *testing.T) {
	r := newAuthStatusRouter("")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/status", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rr.Code)
	}
}

// TestAuthStatusRouteIsPublicOnFullRouter confirms that
// /api/v1/auth/status sits OUTSIDE the auth-protected group
// (the deny-all middleware would short-circuit any auth-gated
// route with 401). Mirrors TestMeRouteIsPublicOnFullRouter in
// me_test.go.
func TestAuthStatusRouteIsPublicOnFullRouter(t *testing.T) {
	dir := t.TempDir()
	denyAll := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"code":"auth_missing"}`))
		})
	}
	deps := RouterDeps{
		APIVersion: "test",
		AuthMW:     denyAll,
		ShareDir:   dir,
	}
	r := NewRouter(deps)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/status", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 — /api/v1/auth/status must sit outside the auth group", rr.Code)
	}
}
