package api

import (
	"context"
	"net/http"
	"os"
	"github.com/go-chi/chi/v5"
	"github.com/lucaspdude/rocinante-harness/apps/api/internal/api/middleware"
	"github.com/lucaspdude/rocinante-harness/apps/api/internal/auth"
	"github.com/lucaspdude/rocinante-harness/apps/api/internal/files"
	"github.com/lucaspdude/rocinante-harness/apps/api/internal/health"
	"github.com/lucaspdude/rocinante-harness/apps/api/internal/keystore"
	"github.com/lucaspdude/rocinante-harness/apps/api/internal/omp"
)

// RouterDeps groups the runtime dependencies needed by the api.
type RouterDeps struct {
	MetaLoader    omp.Loader
	Manager       *omp.Manager
	APIVersion    string
	Idempotency   *middleware.IdempotencyCache
	AuthState     *AuthState
	AuthMW        func(http.Handler) http.Handler
	Titles        *titleKey
	ShareDir      string
	ProviderKeys  *keystore.Store
	LoginHandlers *LoginHandlers
	ModelsCatalog *ModelsCatalogHandler
	Projects      *ProjectsHandlers
	Clone         *CloneHandlers
	Files         *files.FilesHandler
	Git           *files.GitHandler
	CliTools      *CliToolsHandler
}


// WrapHandler chains a middleware around an http.HandlerFunc,
// returning an http.HandlerFunc so it fits chi's strict signatures.
func WrapHandler(mw func(http.Handler) http.Handler, h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		mw(http.HandlerFunc(h)).ServeHTTP(w, r)
	}
}

// NewRouter returns a chi router wired with the v1 endpoints.
func NewRouter(deps RouterDeps) http.Handler {
	r := chi.NewRouter()
	r.Get("/api/v1/healthz", health.Handler)
	r.Get("/api/v1/meta", deps.metaHandler())
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

	// PR-01: /api/v1/login/* public routes.
	if deps.LoginHandlers != nil {
		h := deps.LoginHandlers
		if h.Jobs == nil {
			h.Jobs = NewLoginJobs()
		}
		if h.CmdFactory == nil {
			h.CmdFactory = func(ctx context.Context, name string, args []string) CmdIface {
				return defaultCmdFactory(ctx, name, args...)
			}
		}
		r.Get("/api/v1/login/providers", h.LoginProvidersHandler)
		r.Post("/api/v1/login/start/{provider}", WrapHandler(middleware.IdempotencyMiddleware(deps.Idempotency), h.LoginStartHandler))
		r.Get("/api/v1/login/{jobId}/stream", h.LoginStreamHandler)
		r.Post("/api/v1/login/{jobId}/ack", h.LoginAckHandler)
		r.Get("/api/v1/login/{jobId}/status", h.LoginStatusHandler)
	}

// PR-02: public models.dev catalog.
if deps.ModelsCatalog != nil {
	r.Get("/api/v1/models/catalog", deps.ModelsCatalog.ServeHTTP)
}

// PR-03: /api/v1/me is intentionally public so the DirectoryPicker
// can read it before login. The response shape (home / user / host)
// exposes where the harness is installed; acceptable for single-tenant
// LAN use. If a multi-tenant model ships later, gate this behind
// AuthMW — see phase-3 docs/mvp/phase-3-polishing/02-areas/03
// §5.3 (Risks).
r.Get("/api/v1/me", MeHandler)

if deps.AuthState != nil {
	idem := middleware.IdempotencyMiddleware(deps.Idempotency)
	r.Post("/api/v1/login", WrapHandler(idem, LoginHandler(deps.AuthState)))
	r.Post("/api/v1/refresh", WrapHandler(idem, RefreshHandler(deps.AuthState)))
	r.Post("/api/v1/pairing/redeem", WrapHandler(idem, PairingRedeemHandler(deps.AuthState)))
}

	// Auth-protected endpoints. Devices + Logout + Pairing-init were
	// already in the original auth group; we add the rest here.
	if deps.AuthMW != nil {
		r.Group(func(r chi.Router) {
			r.Use(deps.AuthMW)
			r.Get("/api/v1/devices", DevicesHandler(deps.AuthState))
			r.Delete("/api/v1/devices/{id}", DeleteDeviceHandler(deps.AuthState))
			r.Post("/api/v1/logout", LogoutHandler(deps.AuthState))
			r.Post("/api/v1/pairing/init", PairingInitHandler(deps.AuthState))

			// PR-03 + PR-04: projects + clone.
			if deps.Projects != nil {
				r.Get("/api/v1/projects", deps.Projects.ProjectsHandler)
				r.Post("/api/v1/projects", deps.Projects.ProjectsHandler)
				r.Patch("/api/v1/projects", deps.Projects.PatchHandler)
				r.Delete("/api/v1/projects", deps.Projects.DeleteHandler)
				if deps.Clone != nil {
					r.Post("/api/v1/projects/clone", deps.Clone.CloneStartHandler)
					r.Get("/api/v1/projects/clone/{jobId}/stream", deps.Clone.CloneStreamHandler)
					r.Get("/api/v1/projects/clone/{jobId}/status", deps.Clone.CloneStatusHandler)
				}
				// PR-07: bulk archive / delete from disk.
				r.Post("/api/v1/projects/bulk", deps.Projects.BulkHandler)
			}
			// PR-07: file + git.
			if deps.Files != nil {
				r.Get("/api/v1/files", deps.Files.ListHandler)
				r.Get("/api/v1/files/content", deps.Files.ContentHandler)
				r.Patch("/api/v1/files/content", deps.Files.WriteHandler)
				// PR-02: folder picker mount.
				r.Get("/api/v1/cwd/browse", deps.Files.BrowseHandler)
				// PR-06: ripgrep-backed search across project tree.
				r.Post("/api/v1/search", deps.Files.SearchHandler)
			}
			// PR-06: CLI tools install + device-code login.
			if deps.CliTools != nil {
				ct := deps.CliTools
				r.Get("/api/v1/cli-tools", ct.ListHandler)
				r.Get("/api/v1/cli-tools/{id}/status", ct.StatusHandler)
				r.Post("/api/v1/cli-tools/{id}/install", ct.InstallHandler)
				r.Get("/api/v1/cli-tools/{id}/install/{jobId}/stream", ct.InstallStreamHandler)
				r.Post("/api/v1/cli-tools/{id}/login/start", ct.LoginStartHandler)
				r.Post("/api/v1/cli-tools/{id}/login/{jobId}/ack", ct.AckLoginHandler)
			}

		})
	}

	idem := middleware.IdempotencyMiddleware(deps.Idempotency)
	titles := deps.Titles
	if titles == nil {
		titles = newTitleStore()
	}
	var registryForSession ProjectsLister
	if deps.Projects != nil {
		registryForSession = deps.Projects.Registry
	}
	r.Route("/api/v1/sessions", func(r chi.Router) {
		r.Post("/", CreateSessionHandler(deps.Manager, registryForSession))
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

// metaHandler returns the /api/v1/meta http.Handler.
func (d RouterDeps) metaHandler() http.HandlerFunc {
	var rows []omp.MetaProviderInfo
	if d.LoginHandlers != nil && d.LoginHandlers.Providers != nil {
		for _, p := range d.LoginHandlers.Providers.Snapshot() {
			rows = append(rows, omp.MetaProviderInfo{
				ID:            p.ID,
				Name:          p.Name,
				Available:     p.Available,
				Authenticated: p.Authenticated,
				HelpURL:       p.HelpURL,
			})
		}
	}
	return omp.NewMetaHandler(d.MetaLoader, d.APIVersion, rows, resolveDefaultModel(d.ModelsCatalog))
}

// resolveDefaultModel picks the model id advertised in /api/v1/meta.
// Priority:
//  1. OMP_DEFAULT_MODEL env var (allows operators to override).
//  2. First selectable model from the catalog handler.
//
// Returns "" when neither source yields a value. Used by metaHandler
// so PR-02's client-side persistence has a server-side seed.
func resolveDefaultModel(mc *ModelsCatalogHandler) string {
	if v := os.Getenv("OMP_DEFAULT_MODEL"); v != "" {
		return v
	}
	if mc == nil {
		return ""
	}
	return mc.FirstSelectableID()
}
