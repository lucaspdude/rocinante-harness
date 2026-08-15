package middleware

import (
	"database/sql"
	"net/http"
	"strconv"
	"time"
)

// PersistentRateLimit is the SQLite-backed mirror of the
// in-memory token bucket. The in-memory bucket remains the fast
// path; this layer adds durable counters and a janitor that
// periodically resets counts that have expired.
type PersistentRateLimit struct {
	db      *sql.DB
	Janitor chan struct{}
}

// NewPersistentRateLimit returns the limiter.
func NewPersistentRateLimit(db *sql.DB) *PersistentRateLimit {
	return &PersistentRateLimit{db: db, Janitor: make(chan struct{}, 1)}
}

// Migration returns the SQL the api applies at startup.
func (r *PersistentRateLimit) Migration() string {
	return `
CREATE TABLE IF NOT EXISTS rate_limits (
  scope     TEXT NOT NULL,
  key       TEXT NOT NULL,
  count     INTEGER NOT NULL,
  reset_at  TEXT NOT NULL,
  PRIMARY KEY (scope, key)
);`
}

// Hit records an attempt and returns true if the limit is not
// exhausted. The caller should pair it with a periodic janitor.
func (r *PersistentRateLimit) Hit(scope, key string, limit int, window time.Duration) (bool, error) {
	now := time.Now().UTC()
	row := r.db.QueryRow(`SELECT count, reset_at FROM rate_limits WHERE scope = ? AND key = ?`, scope, key)
	var count int
	var resetStr string
	switch err := row.Scan(&count, &resetStr); err {
	case sql.ErrNoRows:
		count = 0
		resetStr = now.Add(window).Format(time.RFC3339)
		if _, err := r.db.Exec(`INSERT INTO rate_limits(scope, key, count, reset_at) VALUES (?, ?, 1, ?)`, scope, key, resetStr); err != nil {
			return false, err
		}
		return true, nil
	case nil:
		reset, _ := time.Parse(time.RFC3339, resetStr)
		if now.After(reset) {
			count = 0
			resetStr = now.Add(window).Format(time.RFC3339)
		}
		count++
		if _, err := r.db.Exec(`UPDATE rate_limits SET count = ?, reset_at = ? WHERE scope = ? AND key = ?`,
			count, resetStr, scope, key); err != nil {
			return false, err
		}
	default:
		return false, err
	}
	if count > limit {
		return false, nil
	}
	return true, nil
}

// Janitor runs a sweep that zeroes out counters whose reset_at is
// in the past. Called periodically by the api loop.
func (r *PersistentRateLimit) JanitorRun() {
	rows, err := r.db.Query(`SELECT scope, key FROM rate_limits WHERE reset_at < ?`, time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var scope, key string
		if err := rows.Scan(&scope, &key); err != nil {
			continue
		}
		_, _ = r.db.Exec(`UPDATE rate_limits SET count = 0, reset_at = ? WHERE scope = ? AND key = ?`,
			time.Now().UTC().Add(time.Hour).Format(time.RFC3339), scope, key)
	}
}

// RateLimitMiddlewareSQL returns the HTTP middleware that uses
// the persistent rate limiter.
func RateLimitMiddlewareSQL(r *PersistentRateLimit, action string, limit int, window time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ip := ClientIP(req)
			ok, _ := r.Hit(action, ip, limit, window)
			if !ok {
				w.Header().Set("Retry-After", strconv.Itoa(int(window.Seconds())))
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(`{"code":"rate_limited"}`))
				return
			}
			next.ServeHTTP(w, req)
		})
	}
}
