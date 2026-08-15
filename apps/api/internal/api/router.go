// Package api is the HTTP router.
package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/lucaspdude/rocinante-harness/apps/api/internal/api/middleware"
	"github.com/lucaspdude/rocinante-harness/apps/api/internal/auth"
	"github.com/lucaspdude/rocinante-harness/apps/api/internal/health"
	"github.com/lucaspdude/rocinante-harness/apps/api/internal/omp"
)

// RouterDeps groups the runtime dependencies needed by the api.
type RouterDeps struct {
	MetaLoader  omp.Loader
	Manager     *omp.Manager
	APIVersion  string
	Idempotency *middleware.IdempotencyCache
	AuthState   *AuthState
	AuthMW      func(http.Handler) http.Handler
}

// WrapHandler chains a middleware around an http.HandlerFunc,
// returning an http.HandlerFunc so it fits chi's strict signatures.
func WrapHandler(mw func(http.Handler) http.Handler, h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		mw(h).ServeHTTP(w, r)
	}
}

// NewRouter returns a chi router wired with the v1 endpoints.
func NewRouter(deps RouterDeps) http.Handler {
	r := chi.NewRouter()

	r.Get("/api/v1/healthz", health.Handler)
	r.Get("/api/v1/meta", omp.NewMetaHandler(deps.MetaLoader, deps.APIVersion))

	// Auth endpoints (no auth required for login/refresh/pairing).
	if deps.AuthState != nil {
		idem := middleware.IdempotencyMiddleware(deps.Idempotency)
		r.Post("/api/v1/login", WrapHandler(idem, LoginHandler(deps.AuthState)))
		r.Post("/api/v1/refresh", WrapHandler(idem, RefreshHandler(deps.AuthState)))
		r.Post("/api/v1/pairing/redeem", WrapHandler(idem, PairingRedeemHandler(deps.AuthState)))
	}

	// Authenticated endpoints (only mounted when auth is configured).
	if deps.AuthMW != nil {
		r.Group(func(r chi.Router) {
			r.Use(deps.AuthMW)
			r.Get("/api/v1/devices", DevicesHandler(deps.AuthState))
			r.Delete("/api/v1/devices/{id}", DeleteDeviceHandler(deps.AuthState))
			r.Post("/api/v1/logout", LogoutHandler(deps.AuthState))
			r.Post("/api/v1/pairing/init", PairingInitHandler(deps.AuthState))
		})
	}

	// Session endpoints (no auth in P5; P11 adds onboarding).
	idem := middleware.IdempotencyMiddleware(deps.Idempotency)
	r.Route("/api/v1/sessions", func(r chi.Router) {
		r.Post("/", CreateSessionHandler(deps.Manager))
		r.Get("/{id}/events", StreamSessionHandler(deps.Manager))
		r.Post("/{id}/prompt", WrapHandler(idem, PromptHandler(deps.Manager)))
		r.Post("/{id}/abort", WrapHandler(idem, AbortHandler(deps.Manager)))
		r.Post("/{id}/fork", WrapHandler(idem, ForkHandler(deps.Manager)))
	})

	_ = auth.ErrPassphraseMismatch
	return r
}
