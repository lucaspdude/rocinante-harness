// Package api is the HTTP router.
package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/lucaspdude/rocinante-harness/apps/api/internal/api/middleware"
	"github.com/lucaspdude/rocinante-harness/apps/api/internal/health"
	"github.com/lucaspdude/rocinante-harness/apps/api/internal/omp"
)

// RouterDeps groups the runtime dependencies needed by the api.
type RouterDeps struct {
	MetaLoader  omp.Loader
	Manager     *omp.Manager
	APIVersion  string
	Idempotency *middleware.IdempotencyCache
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

	idem := middleware.IdempotencyMiddleware(deps.Idempotency)

	r.Route("/api/v1/sessions", func(r chi.Router) {
		r.Post("/", CreateSessionHandler(deps.Manager))
		r.Get("/{id}/events", StreamSessionHandler(deps.Manager))
		r.Post("/{id}/prompt", WrapHandler(idem, PromptHandler(deps.Manager)))
		r.Post("/{id}/abort", WrapHandler(idem, AbortHandler(deps.Manager)))
		r.Post("/{id}/fork", WrapHandler(idem, ForkHandler(deps.Manager)))
	})

	return r
}
