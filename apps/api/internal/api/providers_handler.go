package api

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/lucaspdude/rocinante-harness/apps/api/internal/keystore"
	"github.com/lucaspdude/rocinante-harness/apps/api/internal/omp"
)

// ProvidersHandler returns POST/DELETE on
// /api/v1/providers/{name}/key. The {name} segment must be one
// of the known providers (anthropic, openai, gemini, openrouter,
// minimax). Unknown names get 400.
//
// POST stores the key on disk (chmod 0600 under share-dir). The
// api picks it up on the next omp session spawn without needing
// a process restart. DELETE clears the entry.
//
// The route sits behind deps.AuthMW (the same middleware that
// protects /api/v1/devices, /api/v1/logout, etc.) so only
// authenticated users can write keys. GET is intentionally not
// exposed — /api/v1/meta reports boolean status, not values.
type ProvidersHandler struct {
	Store   *keystore.Store
	OMP     OMPKiller
	Models  *omp.ModelsConfigWriter
	// Cache, when non-nil, is invalidated after every keystore
	// write so the next /meta or /login/providers request
	// returns fresh data instead of a stale 5-second-TTL
	// snapshot. The cache itself is defined in
	// login_providers.go (LoginProvidersCache.Invalidate).
	Cache   LoginProvidersInvalidator
}

// LoginProvidersInvalidator is the minimum surface
// ProvidersHandler needs from the LoginProvidersCache. The
// concrete *LoginProvidersCache type satisfies this.
type LoginProvidersInvalidator interface {
	Invalidate()
}

// OMPKiller is the minimum surface we need from the omp.Manager:
// CloseAll() that the handler invokes when a key changes. Defined
// here (not imported) so providers_handler_test.go can stub it.
type OMPKiller interface {
	CloseAll()
}

func (h *ProvidersHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if name == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{
			Code:    "bad_request",
			Message: "missing provider name",
		})
		return
	}
	if !keystore.IsKnown(name) {
		writeJSON(w, http.StatusBadRequest, errorResponse{
			Code:    "unknown_provider",
			Message: "unknown provider: " + name,
		})
		return
	}
	p := keystore.ProviderName(name)

	switch r.Method {
	case http.MethodPost:
		h.setKey(w, r, p)
	case http.MethodDelete:
		h.deleteKey(w, r, p)
	default:
		w.Header().Set("Allow", "POST, DELETE")
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{
			Code:    "method_not_allowed",
			Message: "use POST to set, DELETE to clear",
		})
	}
}

type setKeyRequest struct {
	Key string `json:"key"`
}

func (h *ProvidersHandler) setKey(w http.ResponseWriter, r *http.Request, p keystore.ProviderName) {
	var req setKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{
			Code:    "bad_request",
			Message: err.Error(),
		})
		return
	}
	if req.Key == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{
			Code:    "empty_key",
			Message: "key must be a non-empty string",
		})
		return
	}
	if len(req.Key) > 4096 {
		writeJSON(w, http.StatusBadRequest, errorResponse{
			Code:    "key_too_long",
			Message: "key exceeds 4096 chars",
		})
		return
	}
	if err := h.Store.Set(p, req.Key); err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{
			Code:    "keystore_write_failed",
			Message: err.Error(),
		})
		return
	}
	// Phase-6 PR-1: rewrite ~/.omp/agent/models.yml so the OMP
	// subprocess sees the new provider on its next spawn, then kill
	// any in-flight sessions so the next request gets a fresh OMP.
	if h.Models != nil {
		if err := h.Models.SyncIfConfigured(h.Store); err != nil && !errors.Is(err, omp.ErrNotConfigured) {
			log.Printf("providers: models.yml sync failed: %v", err)
		}
	}
	if h.OMP != nil {
		h.OMP.CloseAll()
	}
	// Phase 8 — item 01: invalidate the LoginProvidersCache so
	// the next /meta or /login/providers request rebuilds the
	// provider list from omp's get_login_providers (which now
	// sees the freshly-written models.yml / MINIMAX_API_KEY).
	// Without this, the cache would serve stale "authenticated:
	// false" data for up to 5 s after the write.
	if h.Cache != nil {
		h.Cache.Invalidate()
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"provider": string(p),
		"stored":   true,
	})
}

func (h *ProvidersHandler) deleteKey(w http.ResponseWriter, _ *http.Request, p keystore.ProviderName) {
	if err := h.Store.Delete(p); err != nil {
		// Treat as success: the file may simply not have a key
		// for this provider. Idempotent DELETE.
		if !errors.Is(err, keystore.ErrNotFound) {
			writeJSON(w, http.StatusInternalServerError, errorResponse{
				Code:    "keystore_write_failed",
				Message: err.Error(),
			})
			return
		}
	}
	// Phase-6 PR-1: same as setKey — refresh models.yml and kill
	// any active OMP so the deletion is reflected immediately.
	if h.Models != nil {
		_ = h.Models.SyncIfConfigured(h.Store) // best-effort
	}
	if h.OMP != nil {
		h.OMP.CloseAll()
	}
	if h.Cache != nil {
		h.Cache.Invalidate()
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"provider": string(p),
		"stored":   false,
	})
}
