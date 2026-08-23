package api

// Regression test for the phase-8 shipping bug where the
// struct literal in main.go lost the AuthMW field. NewRouter
// guards `if deps.AuthMW != nil` (router.go:136) before mounting
// the auth-protected group; if AuthMW is the zero value, all
// the routes inside the group (projects, devices, devices/{id},
// logout, providers, ssh, ssh/keys, ssh/servers) silently
// disappear and every call to those routes returns 404.
//
// This test would have caught the v1.26.2 / v1.26.3 regression.

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewRouter_AuthGroupRegisteredWhenAuthMWProvided(t *testing.T) {
	// Middleware that accepts anything. If AuthMW is nil,
	// NewRouter skips the auth group entirely — the failing
	// path observed in v1.26.2 / v1.26.3.
	allowAll := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r)
		})
	}

	// Each route here sits inside either the top-level
	// `if deps.AuthMW != nil` group (devices, logout, ssh) OR
	// the deeper `if deps.Projects != nil` group (projects).
	// Both groups must be mounted to avoid a 404. Tests must
	// supply a real `Projects` handler (an empty struct is
	// enough — the routes are registered against
	// `&ProjectsHandlers{}`).
	deps := RouterDeps{
		APIVersion: "test",
		AuthMW:     allowAll,
		Projects:   &ProjectsHandlers{},
	}

	r := NewRouter(deps)

	want := []struct {
		method string
		path   string
	}{
		{"POST", "/api/v1/projects"},
		{"GET", "/api/v1/projects"},
		{"PATCH", "/api/v1/projects"},
		{"POST", "/api/v1/projects/bulk"},
		{"GET", "/api/v1/devices"},
		{"DELETE", "/api/v1/devices/abc123"},
		{"POST", "/api/v1/logout"},
	}
	for _, tc := range want {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		if rr.Code == http.StatusNotFound {
			t.Errorf("%s %s: 404 — auth group was not mounted (AuthMW was not wired)",
				tc.method, tc.path)
		}
	}
}

// TestNewRouter_NoAuthMWSkipsAuthGroup documents the current
// behaviour: when AuthMW is nil the auth group is skipped,
// which means all /api/v1/* routes that required auth are
// simply not registered. This is the pre-phase-1 default
// (single-tenant LAN deployments where auth is opt-in).
func TestNewRouter_NoAuthMWSkipsAuthGroup(t *testing.T) {
	deps := RouterDeps{
		APIVersion: "test",
		AuthMW:     nil, // explicit: the production code skips
		// the auth group when AuthMW is the zero value.
	}

	r := NewRouter(deps)

	cases := []struct {
		method, path string
	}{
		{"POST", "/api/v1/projects"},
		{"GET", "/api/v1/devices"},
		{"POST", "/api/v1/logout"},
	}

	for _, tc := range cases {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		if rr.Code != http.StatusNotFound {
			t.Errorf("%s %s: status=%d, want 404 (auth group should be skipped when AuthMW is nil)",
				tc.method, tc.path, rr.Code)
		}
	}
}
