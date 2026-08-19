package api

import (
	"net/http"
	"os"
)

// meResponse is the wire shape of GET /api/v1/me. The web client
// uses it to expand "~" in folder paths and to render the
// picker's breadcrumb without leaking server-side env vars.
type meResponse struct {
	Home string `json:"home"`
	User string `json:"user"`
	Host string `json:"host"`
}

// MeHandler returns the host's home directory + current user +
// hostname. Read-only; safe to expose behind auth (no secrets
// returned). On systemd-less systems where $HOME is unset, the
// api binary falls back to "/root" so the picker still resolves.
func MeHandler(w http.ResponseWriter, r *http.Request) {
	home, _ := os.UserHomeDir()
	if home == "" {
		home = "/root"
	}
	user := os.Getenv("USER")
	if user == "" {
		user = "root"
	}
	host, _ := os.Hostname()
	writeJSON(w, http.StatusOK, meResponse{Home: home, User: user, Host: host})
}
