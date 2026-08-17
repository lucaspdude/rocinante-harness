package api

import (
	"net/http"
	"os"
)

// OnboardingStatusResponse is the body of GET /api/v1/onboarding/status.
type OnboardingStatusResponse struct {
	Initialized   bool   `json:"initialized"`
	RequiresSetup bool   `json:"requires_setup"`
	APIVersion    string `json:"api_version"`
}

// OnboardingStatus reports whether the api has been initialized
// (passphrase-wrapped key + SQLite). The front uses this to
// decide between /login and /onboarding/wizard.
//
// The signal is "does .ed25519 exist on disk?". We deliberately
// do NOT try to parse the key here: the api's main loop is the
// only code that knows the passphrase (from the ROCINANTE_PASSPHRASE
// env var), and parsing an encrypted envelope requires that
// passphrase. A status handler that tried to parse would have to
// either (a) duplicate the env-reading logic or (b) accept an
// empty passphrase and incorrectly conclude the api is
// uninitialized whenever the installer pre-initialized the key.
func OnboardingStatus(shareDir string, apiVersion string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		ed25519Path := shareDir + "/.ed25519"
		_, err := os.Stat(ed25519Path)
		initialized := err == nil
		writeJSON(w, http.StatusOK, OnboardingStatusResponse{
			Initialized:   initialized,
			RequiresSetup: !initialized,
			APIVersion:    apiVersion,
		})
	}
}
