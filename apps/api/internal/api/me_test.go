package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

// newPublicMeRouter mounts only the public /api/v1/me route
// to confirm the registration sits outside the auth-protected
// group (PR-03 D6). The RouterDeps with AuthMW wires the
// protected group, so we use a minimal chi router here.
func newPublicMeRouter() http.Handler {
	r := chi.NewRouter()
	r.Get("/api/v1/me", MeHandler)
	return r
}

func TestMeHandlerPublic(t *testing.T) {
	r := newPublicMeRouter()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rr.Code, rr.Body.String())
	}
	var body struct {
		Home string `json:"home"`
		User string `json:"user"`
		Host string `json:"host"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Home == "" {
		t.Errorf("home field empty")
	}
	if body.User == "" {
		t.Errorf("user field empty")
	}
	if body.Host == "" {
		t.Errorf("host field empty")
	}
}

// TestMeRouteIsPublicOnFullRouter confirms that when the full
// RouterDeps is mounted with AuthMW wired (no caller is
// authenticated), GET /api/v1/me still returns 200. PR-03 D6
// moved the route out of the auth-protected group; this test
// is the regression guard.
func TestMeRouteIsPublicOnFullRouter(t *testing.T) {
	// Stub auth middleware that always rejects — proves the
	// /api/v1/me route sits OUTSIDE the protected group, because
	// a 401 from the middleware would short-circuit the request.
	denyAll := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"code":"auth_missing"}`))
		})
	}
	deps := RouterDeps{
		APIVersion: "test",
		AuthMW:     denyAll,
	}
	r := NewRouter(deps)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 — /api/v1/me must sit outside the auth group", rr.Code)
	}
}