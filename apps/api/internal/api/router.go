package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/lucaspdude/rocinante-harness/apps/api/internal/api/middleware"
	"github.com/lucaspdude/rocinante-harness/apps/api/internal/auth"
	"github.com/lucaspdude/rocinante-harness/apps/api/internal/health"
	"github.com/lucaspdude/rocinante-harness/apps/api/internal/keystore"
	"github.com/lucaspdude/rocinante-harness/apps/api/internal/omp"
)

// RouterDeps groups the runtime dependencies needed by the api.
type RouterDeps struct {
	MetaLoader   omp.Loader
	Manager      *omp.Manager
	APIVersion   string
	Idempotency  *middleware.IdempotencyCache
	AuthState    *AuthState
	AuthMW       func(http.Handler) http.Handler
	Titles       *titleKey
	ShareDir     string
	ProviderKeys *keystore.Store
}

// WrapHandler chains a middleware around an http.HandlerFunc,
// returning an http.HandlerFunc so it fits chi's strict signatures.
func WrapHandler(mw func(http.Handler) http.Handler, h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		mw(h).ServeHTTP(w, r)
	}
}

// NewRouter returns a chi router wired with the v1 endpoints.
//
// Route visibility (post v0.6.5):
//
//   Public (no auth):
//     GET  /api/v1/healthz
//     GET  /api/v1/meta                       (booleans only)
//     GET  /api/v1/onboarding/status         (file presence only)
//     POST /api/v1/onboarding/init           (creates .ed25519)
//     POST /api/v1/providers/{name}/key     (writes the keystore)
//     DELETE /api/v1/providers/{name}/key
//     POST /api/v1/login / refresh / pairing/redeem
//
//   Authenticated:
//     GET    /api/v1/devices
//     DELETE /api/v1/devices/{id}
//     POST   /api/v1/logout
//     POST   /api/v1/pairing/init
//     POST   /api/v1/sessions/  (and all /api/v1/sessions/{id}/*)
//
// The provider key routes are public because the onboarding
// wizard needs to set a key BEFORE the api has any auth state.
// After onboarding the same routes stay public — the keystore
// is global, not per-user, so there's no auth boundary to
// layer on top.
func NewRouter(deps RouterDeps) http.Handler {
	r := chi.NewRouter()
	r.Get("/api/v1/healthz", health.Handler)
	if deps.ProviderKeys != nil {
		r.Get("/api/v1/meta", omp.NewMetaHandler(deps.MetaLoader, deps.APIVersion, &keystore.EnvProbe{Store: deps.ProviderKeys}))
	} else {
		// Backward compat: the meta handler is required when
		// the keystore is missing. This branch is dead in
		// production but keeps tests simple.
		r.Get("/api/v1/meta", omp.NewMetaHandler(deps.MetaLoader, deps.APIVersion, &keystore.EnvProbe{}))
	}
	r.Get("/api/v1/onboarding/status", OnboardingStatus(deps.ShareDir, deps.APIVersion))
	if deps.ShareDir != "" {
		r.Post("/api/v1/onboarding/init", OnboardingInit(deps.ShareDir))
	}
	if deps.ProviderKeys != nil {
		ph := &ProvidersHandler{Store: deps.ProviderKeys}
		r.Route("/api/v1/providers", func(r chi.Router) {
			r.Post("/{name}/key", ph.ServeHTTP)
			r.Delete("/{name}/key", ph.ServeHTTP)
		})
	}

	if deps.AuthState != nil {
		idem := middleware.IdempotencyMiddleware(deps.Idempotency)
		r.Post("/api/v1/login", WrapHandler(idem, LoginHandler(deps.AuthState)))
		r.Post("/api/v1/refresh", WrapHandler(idem, RefreshHandler(deps.AuthState)))
		r.Post("/api/v1/pairing/redeem", WrapHandler(idem, PairingRedeemHandler(deps.AuthState)))
	}

	if deps.AuthMW != nil {
		r.Group(func(r chi.Router) {
			r.Use(deps.AuthMW)
			r.Get("/api/v1/devices", DevicesHandler(deps.AuthState))
			r.Delete("/api/v1/devices/{id}", DeleteDeviceHandler(deps.AuthState))
			r.Post("/api/v1/logout", LogoutHandler(deps.AuthState))
			r.Post("/api/v1/pairing/init", PairingInitHandler(deps.AuthState))
		})
	}

	idem := middleware.IdempotencyMiddleware(deps.Idempotency)
	titles := deps.Titles
	if titles == nil {
		titles = newTitleStore()
	}
	r.Route("/api/v1/sessions", func(r chi.Router) {
		r.Post("/", CreateSessionHandler(deps.Manager))
		r.Get("/", SessionsListHandler(deps.Manager, titles))
		r.Get("/{id}/events", StreamSessionHandler(deps.Manager))
		r.Post("/{id}/prompt", WrapHandler(idem, PromptHandler(deps.Manager)))
		r.Post("/{id}/abort", WrapHandler(idem, AbortHandler(deps.Manager)))
		r.Post("/{id}/fork", WrapHandler(idem, ForkHandler(deps.Manager)))
		r.Post("/{id}/title", SessionTitleHandler(deps.Manager, titles))
		r.Delete("/{id}", SessionDeleteHandler(deps.Manager, titles))
	})

	_ = auth.ErrPassphraseMismatch
	return r
}
