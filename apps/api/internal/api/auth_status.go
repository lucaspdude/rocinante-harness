package api

import (
	"net/http"
	"os"
)

// AuthStatusResponse is the wire shape of GET /api/v1/auth/status.
//
// initialized     — api has an Ed25519 key on disk (passphrase auth is enabled).
// auth_required   — == initialized on the api side; web reads this as the
//                  "is login enforced?" flag. When false, the api is in
//                  onboarding mode and login is not a valid flow.
// device_known    — browser sent the `rh-device-id` cookie from a prior
//                  successful sign-in. Used by the web to decide between
//                  the "Sign in" CTA (first visit) and a redirect to
//                  /login (returning user).
type AuthStatusResponse struct {
	Initialized   bool `json:"initialized"`
	AuthRequired  bool `json:"auth_required"`
	DeviceKnown   bool `json:"device_known"`
}

// AuthStatusHandler returns the public auth status for the web's
// unauthed route (Phase 7 — item 01). The endpoint is intentionally
// public so the home page can read it before the user has a token.
// No secrets are returned; the boolean flags only.
//
// 503 is reserved for share-dir-unreadable / internal errors; the
// 200 path always reflects real api state (initialized=false is a
// normal "fresh install" state, not an error).
func AuthStatusHandler(shareDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if shareDir == "" {
			// 503 with a stable error code; the web treats this
			// as a fallback to "first visit" via the safe default
			// in lib/auth/auth-status.ts.
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{
				"code":    "internal",
				"message": "share dir not configured",
			})
			return
		}
		ed25519Path := shareDir + "/.ed25519"
		_, err := os.Stat(ed25519Path)
		initialized := err == nil

		_, cookieErr := r.Cookie("rh-device-id")
		deviceKnown := cookieErr == nil

		writeJSON(w, http.StatusOK, AuthStatusResponse{
			Initialized:  initialized,
			AuthRequired: initialized,
			DeviceKnown:  deviceKnown,
		})
	}
}
