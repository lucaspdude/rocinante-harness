package auth

import (
	"encoding/json"
	"net/http"
	"strings"
)

// AuthMiddleware enforces that requests carry an Authorization
// Bearer token. Phase 4 returns 401 auth_missing when the header
// is absent or not a Bearer token. Phase 5 replaces the body with
// full Ed25519 verification + device lookup.
func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		if header == "" {
			writeAuthMissing(w)
			return
		}
		if !strings.HasPrefix(header, "Bearer ") {
			writeAuthMissing(w)
			return
		}
		// P5 will verify the signature here and look up the
		// device. For now we just pass through.
		next.ServeHTTP(w, r)
	})
}

func writeAuthMissing(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(map[string]string{"code": "auth_missing"})
}
