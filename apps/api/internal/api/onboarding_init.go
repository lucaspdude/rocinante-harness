package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/lucaspdude/rocinante-harness/apps/api/internal/auth"
	"github.com/lucaspdude/rocinante-harness/apps/api/internal/storage"
)

// OnboardingInitRequest is the body of POST /api/v1/onboarding/init.
//
// Locale is accepted so the front can persist the user's locale
// choice in the same call that creates the api credentials.
// Currently the locale is just echoed back in the response;
// storing it server-side is a future improvement.
type OnboardingInitRequest struct {
	Passphrase string `json:"passphrase"`
	Locale     string `json:"locale"`
}

// OnboardingInitResponse is the body of a successful
// POST /api/v1/onboarding/init.
type OnboardingInitResponse struct {
	Initialized bool   `json:"initialized"`
	Locale      string `json:"locale"`
}

// minPassphraseLength is the smallest passphrase the front
// accepts. Matches the front's onSubmit() check.
const minPassphraseLength = 8

// OnboardingInit handles POST /api/v1/onboarding/init. The
// route is the "create a brand new install" call: it generates
// a fresh Ed25519 keypair, encrypts it with the user's
// passphrase, and writes .ed25519 + .ed25519.bak + the SQLite
// schema. This is the HTTP equivalent of `api init` and is
// what the web's /onboarding flow calls.
//
// The route is unauthenticated (the api has no credentials yet
// by definition) and is the ONLY POST exposed outside the
// auth group. Locking the api down so that .ed25519 cannot be
// created over the wire once it already exists is the whole
// point of the 409 below.
func OnboardingInit(shareDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req OnboardingInitRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, errorResponse{
				Code:    "bad_request",
				Message: err.Error(),
			})
			return
		}
		if len(req.Passphrase) < minPassphraseLength {
			writeJSON(w, http.StatusBadRequest, errorResponse{
				Code:    "passphrase_too_short",
				Message: "passphrase must be at least 8 characters",
			})
			return
		}
		locale := strings.TrimSpace(req.Locale)
		if locale == "" {
			locale = "en-US"
		}

		ed25519Path := filepath.Join(shareDir, ".ed25519")
		if _, err := os.Stat(ed25519Path); err == nil {
			writeJSON(w, http.StatusConflict, errorResponse{
				Code:    "already_initialized",
				Message: ".ed25519 already exists; refuse to re-initialize",
			})
			return
		}

		if err := os.MkdirAll(shareDir, 0o700); err != nil {
			writeJSON(w, http.StatusInternalServerError, errorResponse{
				Code:    "mkdir_failed",
				Message: err.Error(),
			})
			return
		}

		dbPath := filepath.Join(shareDir, "roc-harness.db")
		db, err := storage.Open(dbPath)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errorResponse{
				Code:    "storage_open_failed",
				Message: err.Error(),
			})
			return
		}
		defer db.Close()
		if err := storage.ApplyMigrations(db); err != nil {
			writeJSON(w, http.StatusInternalServerError, errorResponse{
				Code:    "migrations_failed",
				Message: err.Error(),
			})
			return
		}

		sk, pk, err := auth.NewKeyPair()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errorResponse{
				Code:    "keygen_failed",
				Message: err.Error(),
			})
			return
		}

		// Encrypt with the user's passphrase. Same default KDF
		// params as the `api init` subcommand, so the two paths
		// are interchangeable.
		if err := auth.SaveKeyFileEncrypted(
			ed25519Path, sk, pk, req.Passphrase, auth.DefaultKDFParams,
		); err != nil {
			writeJSON(w, http.StatusInternalServerError, errorResponse{
				Code:    "key_save_failed",
				Message: err.Error(),
			})
			return
		}
		backupPath := filepath.Join(shareDir, ".ed25519.bak")
		_ = auth.SaveKeyFileEncrypted(
			backupPath, sk, pk, req.Passphrase, auth.DefaultKDFParams,
		)
		_ = pk

		writeJSON(w, http.StatusOK, OnboardingInitResponse{
			Initialized: true,
			Locale:      locale,
		})
	}
}
