package auth

import (
	"context"
	"crypto/ed25519"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"
)

// ctxKey is a private type for context keys in auth.
type ctxKey int

const (
	ctxDeviceID ctxKey = iota
)

// PublicKeyLoader is the seam used by the middleware to look up
// the device's public key for verification.
type PublicKeyLoader interface {
	PublicKey() ed25519.PublicKey
}

// RevocationCache holds recently revoked devices so the middleware
// can short-circuit without a database round-trip.
type RevocationCache struct {
	mu        sync.RWMutex
	revoked   map[string]time.Time
	pkLoader  PublicKeyLoader
	db        *sql.DB
	stop      chan struct{}
	refreshes time.Duration
}

// NewRevocationCache returns a cache that refreshes from the
// database every 5 seconds.
func NewRevocationCache(db *sql.DB, pkLoader PublicKeyLoader) *RevocationCache {
	c := &RevocationCache{
		revoked:   make(map[string]time.Time),
		pkLoader:  pkLoader,
		db:        db,
		stop:      make(chan struct{}),
		refreshes: 5 * time.Second,
	}
	go c.run()
	return c
}

// Stop halts the background refresh loop.
func (c *RevocationCache) Stop() {
	close(c.stop)
}

func (c *RevocationCache) run() {
	c.refresh()
	t := time.NewTicker(c.refreshes)
	defer t.Stop()
	for {
		select {
		case <-c.stop:
			return
		case <-t.C:
			c.refresh()
		}
	}
}

func (c *RevocationCache) refresh() {
	rows, err := c.db.Query(`SELECT id, revoked_at FROM devices WHERE revoked_at IS NOT NULL`)
	if err != nil {
		return
	}
	defer rows.Close()
	c.mu.Lock()
	defer c.mu.Unlock()
	c.revoked = make(map[string]time.Time)
	for rows.Next() {
		var id, ra string
		if err := rows.Scan(&id, &ra); err != nil {
			continue
		}
		t, _ := time.Parse(time.RFC3339, ra)
		c.revoked[id] = t
	}
}

// IsRevoked returns true if the device id is in the revocation set.
func (c *RevocationCache) IsRevoked(deviceID string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, ok := c.revoked[deviceID]
	return ok
}

// AuthMiddleware returns the middleware that validates Bearer
// tokens. The pkLoader supplies the public key for Ed25519
// verification; the revCache short-circuits revoked devices.
func AuthMiddleware(pkLoader PublicKeyLoader, revCache *RevocationCache) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if header == "" {
				writeAuthError(w, http.StatusUnauthorized, "auth_missing")
				return
			}
			if !strings.HasPrefix(header, "Bearer ") {
				writeAuthError(w, http.StatusUnauthorized, "auth_missing")
				return
			}
			token := strings.TrimPrefix(header, "Bearer ")
			claims, err := VerifyAccessToken(token, pkLoader.PublicKey())
			if err != nil {
				if errors.Is(err, errors.New("auth_token_expired")) {
					writeAuthError(w, http.StatusUnauthorized, "auth_token_expired")
					return
				}
				writeAuthError(w, http.StatusUnauthorized, "auth_invalid_token")
				return
			}
			if revCache != nil && revCache.IsRevoked(claims.Subject) {
				writeAuthError(w, http.StatusUnauthorized, "auth_device_revoked")
				return
			}
			ctx := context.WithValue(r.Context(), ctxDeviceID, claims.Subject)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// DeviceIDFromContext returns the device id stored in the request
// context by AuthMiddleware, or "" if not present.
func DeviceIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(ctxDeviceID).(string); ok {
		return v
	}
	return ""
}

func writeAuthError(w http.ResponseWriter, code int, body string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"code": body})
}
