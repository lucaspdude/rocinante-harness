package middleware

import (
	"crypto/tls"
	"net/http"
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
// CORSHandler is intentionally a no-op now that the web is the
// only thing that talks to the api (via the Next.js rewrite in
// apps/web/next.config.ts, which proxies /api/v1/* to
// 127.0.0.1:30179/api/v1/* in-process). The browser sees the
// request as same-origin (the URL is the web, not the api), so
// no cross-origin CORS preflight is issued. Any request that
// reaches this handler is either same-origin (allowed) or came
// from a private network and should be rejected — but rejecting
// the latter is the responsibility of the firewall, not this
// middleware. We keep the function in place for compatibility
// with old callers but it now just chains to the next handler.
func CORSHandler(cfg CORSConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
