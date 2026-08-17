package api

// Public models catalog endpoint (PR-02). Returns models.dev's
// catalog crossed with omp's login_providers (selectable annotation).
// Public because the catalog is reference info, no auth needed.

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/lucaspdude/rocinante-harness/apps/api/internal/catalog"
)

// ModelsCatalogHandler is GET /api/v1/models/catalog. The catalog
// is fetched in background; first request blocks briefly while the
// cache warms up.
type ModelsCatalogHandler struct {
	Catalog         *catalog.ModelsDevCatalog
	LoginProviders  LoginProvidersProvider
	MaxQueryLength  int
	MaxLimit        int
}

// NewModelsCatalogHandler returns a handler with sensible defaults.
func NewModelsCatalogHandler(c *catalog.ModelsDevCatalog, lp LoginProvidersProvider) *ModelsCatalogHandler {
	return &ModelsCatalogHandler{
		Catalog:        c,
		LoginProviders: lp,
		MaxQueryLength: 120,
		MaxLimit:       50,
	}
}

func (h *ModelsCatalogHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{
			Code:    "method_not_allowed",
			Message: "use GET",
		})
		return
	}
	q := r.URL.Query().Get("q")
	if len(q) > h.MaxQueryLength {
		writeJSON(w, http.StatusBadRequest, errorResponse{
			Code:    "query_too_long",
			Message: "q exceeds 120 chars",
		})
		return
	}
	providerFilter := r.URL.Query().Get("provider")
	selectableOnly := true
	if raw := r.URL.Query().Get("selectable"); raw != "" {
		if b, err := strconv.ParseBool(raw); err == nil {
			selectableOnly = b
		}
	}
	limit := 10
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 && v <= h.MaxLimit {
			limit = v
		}
	}

	entries := h.Catalog.Snapshot()
	var providers LoginProvidersProvider
	if h.LoginProviders != nil {
		providers = h.LoginProviders
	}
	if providers != nil && len(entries) > 0 {
		// Pull the providers on every request: small, in-process,
		// and lets us annotate `selectable` from the live state.
		annotated := catalog.AnnotateSelectable(append([]catalog.ModelsDevEntry(nil), entries...), providers.List())
		entries = annotated
	}

	results := catalog.Search(entries, q, providerFilter, selectableOnly, limit)
	if results == nil {
		results = []catalog.ModelsDevEntry{}
	}
	stale := h.Catalog.Stale()

	resp := map[string]any{
		"results": results,
		"count":   len(results),
		"stale":   stale,
	}
	if stale {
		w.Header().Set("X-Catalog-Stale", "1")
		w.WriteHeader(http.StatusOK)
	}
	w.Header().Set("Content-Type", "application/json")
	if results == nil {
		// Touch json to keep the import alive in trimmed builds.
		_ = json.Valid(nil)
	}
	_ = json.NewEncoder(w).Encode(resp)
}
