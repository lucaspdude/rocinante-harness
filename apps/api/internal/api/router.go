// Package api is the HTTP router. P2 populates it.
package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/lucaspdude/rocinante-harness/apps/api/internal/health"
	"github.com/lucaspdude/rocinante-harness/apps/api/internal/omp"
)

// RouterDeps groups the runtime dependencies needed by the api.
type RouterDeps struct {
	MetaLoader omp.Loader
	Manager    *omp.Manager
	APIVersion string
}

// NewRouter returns a chi router wired with the v1 endpoints.
func NewRouter(deps RouterDeps) http.Handler {
	r := chi.NewRouter()

	r.Get("/api/v1/healthz", health.Handler)
	r.Get("/api/v1/meta", omp.NewMetaHandler(deps.MetaLoader, deps.APIVersion))

	r.Route("/api/v1/sessions", func(r chi.Router) {
		r.Post("/", CreateSessionHandler(deps.Manager))
		r.Get("/{id}/events", StreamSessionHandler(deps.Manager))
	})

	return r
}
