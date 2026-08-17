package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/lucaspdude/rocinante-harness/apps/api/internal/keystore"
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
	Store *keystore.Store
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
	writeJSON(w, http.StatusOK, map[string]any{
		"provider": string(p),
		"stored":   false,
	})
}
