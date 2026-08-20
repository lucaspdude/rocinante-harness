package api

// Public models catalog endpoint (PR-02). Returns models.dev's
// catalog crossed with omp's login_providers (selectable annotation).
// Public because the catalog is reference info, no auth needed.

import (
	"context"
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
	Rates           *catalog.RatesCache
	MaxQueryLength  int
	MaxLimit        int
}

// NewModelsCatalogHandler returns a handler with sensible defaults.
// `rates` may be nil — the handler then skips per-locale conversion
// and serves the canonical USD-only wire format.
func NewModelsCatalogHandler(c *catalog.ModelsDevCatalog, lp LoginProvidersProvider, rates *catalog.RatesCache) *ModelsCatalogHandler {
	return &ModelsCatalogHandler{
		Catalog:        c,
		LoginProviders: lp,
		Rates:          rates,
		MaxQueryLength: 120,
		MaxLimit:       50,
	}
}
// FirstSelectableID returns the id of the first selectable model
// from the catalog (PR-02 fallback when OMP_DEFAULT_MODEL is
// unset). Returns "" when the catalog is empty or no model is
// selectable. Lazy-warms the catalog snapshot so a freshly-booted
// api can answer /api/v1/meta with a meaningful default.
func (h *ModelsCatalogHandler) FirstSelectableID() string {
	if h == nil || h.Catalog == nil {
		return ""
	}
	_ = h.Catalog.Refresh(context.Background())
	entries := h.Catalog.Snapshot()
	if len(entries) == 0 {
		return ""
	}
	if h.LoginProviders != nil {
		entries = catalog.AnnotateSelectable(append([]catalog.ModelsDevEntry(nil), entries...), h.LoginProviders.List())
	}
	for _, e := range entries {
		if e.Selectable {
			return e.ID
		}
	}
	return ""
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
	locale := r.URL.Query().Get("locale")
	if locale == "" {
		locale = "en-US"
	}
	currency := catalog.CurrencyForLocale(locale)

	// Best-effort rate refresh. If the rate service is down we
	// just serve the USD-only response (no per-locale fields).
	// The errors are already logged inside Refresh.
	if h.Rates != nil && currency != "USD" {
		_ = h.Rates.Refresh(r.Context())
	}
	// Lazy-warm the models.dev cache on the first request after
	// api startup so the picker actually has entries. Refresh is
	// serialized via an in-flight channel so concurrent first
	// requests don't hammer models.dev; subsequent requests get
	// the cached snapshot for up to TTL.
	_ = h.Catalog.Refresh(r.Context())
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
	if h.Rates != nil && currency != "USD" {
		for i := range results {
			if in, ok := h.Rates.Convert(results[i].CostInput, currency); ok {
				results[i].CostInputLocal = &in
			}
			if out, ok := h.Rates.Convert(results[i].CostOutput, currency); ok {
				results[i].CostOutputLocal = &out
			}
			if results[i].CostInputLocal != nil || results[i].CostOutputLocal != nil {
				results[i].Currency = currency
			}
		}
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
