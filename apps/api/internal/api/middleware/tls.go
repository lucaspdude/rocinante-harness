package middleware

import (
	"crypto/tls"
	"net/http"
	"strings"
)

// TLSConfig bundles the knobs needed to enable TLS at the api
// edge. CertFile and KeyFile are read once at startup.
type TLSConfig struct {
	CertFile  string
	KeyFile   string
	MinVersion uint16
}

// TLSHandler wraps the next handler with a TLS-aware security
// response header set. Use this when the api is behind a TLS
// reverse proxy (Caddy, Cloudflare Tunnel) and the connection
// to the front is HTTPS.
func TLSHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Strict-Transport-Security", "max-age=15552000; includeSubDomains")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}

// CORSConfig bounds the front origins allowed to call the api.
// An empty allow-list disables CORS (rejects all cross-origin
// requests with 403).
type CORSConfig struct {
	AllowedOrigins []string
}

// CORSHandler enforces the allow-list. Echoes the request Origin
// header when it matches; otherwise returns 403.
func CORSHandler(cfg CORSConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin == "" {
				// Same-origin request — pass through.
				next.ServeHTTP(w, r)
				return
			}
			allowed := false
			for _, o := range cfg.AllowedOrigins {
				if strings.EqualFold(o, origin) {
					allowed = true
					break
				}
			}
			if !allowed {
				http.Error(w, "{\"code\":\"cors_forbidden\"}", http.StatusForbidden)
				return
			}
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Idempotency-Key")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ParseTLSVersion maps a string to a tls.Version constant.
func ParseTLSVersion(s string) uint16 {
	switch s {
	case "1.3":
		return tls.VersionTLS13
	case "1.2":
		return tls.VersionTLS12
	default:
		return tls.VersionTLS12
	}
}
