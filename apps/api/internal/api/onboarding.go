package api

import (
	"net/http"
	"os"

	"github.com/lucaspdude/rocinante-harness/apps/api/internal/auth"
)

// OnboardingStatusResponse is the body of GET /api/v1/onboarding/status.
type OnboardingStatusResponse struct {
	Initialized   bool   `json:"initialized"`
	RequiresSetup bool   `json:"requires_setup"`
	APIVersion    string `json:"api_version"`
}

// OnboardingStatus reports whether the api has been initialized
// (passphrase-wrapped key + SQLite). The front uses this to decide
// between /login and /onboarding/wizard.
func OnboardingStatus(shareDir string, apiVersion string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		ed25519Path := shareDir + "/.ed25519"
		_, err := os.Stat(ed25519Path)
		initialized := err == nil
		// Even if the file exists, validate it can be parsed.
		if initialized {
			_, _, err := auth.LoadKeyFile(ed25519Path, "")
			if err != nil {
				initialized = false
			}
		}
		writeJSON(w, http.StatusOK, OnboardingStatusResponse{
			Initialized:   initialized,
			RequiresSetup: !initialized,
			APIVersion:    apiVersion,
		})
	}
}
