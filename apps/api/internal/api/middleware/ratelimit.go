// Package middleware contains HTTP middlewares for the api.
package middleware

import (
	"net"
	"net/http"
	"sync"
	"time"
)

// RateLimiter is an in-memory token bucket per key.
type RateLimiter struct {
	mu       sync.Mutex
	buckets  map[string]*bucket
	rules    map[string]rule
}

// rule defines the daily quota for a named action.
type rule struct {
	limit  int
	window time.Duration
}

type bucket struct {
	tokens float64
	last   time.Time
}

// NewRateLimiter returns an empty limiter.
func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		buckets: make(map[string]*bucket),
		rules:   make(map[string]rule),
	}
}

// SetRule configures the limit for an action.
func (r *RateLimiter) SetRule(action string, limit int, window time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rules[action] = rule{limit: limit, window: window}
}

// Allow returns true if the key is allowed to perform the action.
// Returns the retry-after duration when not allowed.
func (r *RateLimiter) Allow(action, key string) (bool, time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	rl, ok := r.rules[action]
	if !ok {
		return true, 0
	}
	b, ok := r.buckets[action+"|"+key]
	if !ok {
		b = &bucket{tokens: float64(rl.limit), last: time.Now()}
		r.buckets[action+"|"+key] = b
	}
	now := time.Now()
	elapsed := now.Sub(b.last).Seconds()
	refill := elapsed * (float64(rl.limit) / rl.window.Seconds())
	b.tokens = minFloat(float64(rl.limit), b.tokens+refill)
	b.last = now
	if b.tokens < 1 {
		secondsToNext := (1 - b.tokens) * (rl.window.Seconds() / float64(rl.limit))
		return false, time.Duration(secondsToNext * float64(time.Second))
	}
	b.tokens -= 1
	return true, 0
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// ClientIP returns the client's IP from r.RemoteAddr with a
// fallback to X-Forwarded-For.
func ClientIP(r *http.Request) string {
	if v := r.Header.Get("X-Forwarded-For"); v != "" {
		return v
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// RateLimitMiddleware denies requests when the action quota is
// exhausted.
func RateLimitMiddleware(limiter *RateLimiter, action string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := ClientIP(r)
			ok, retry := limiter.Allow(action, ip)
			if !ok {
				w.Header().Set("Retry-After", formatRetryAfter(retry))
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(`{"code":"rate_limited"}`))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func formatRetryAfter(d time.Duration) string {
	seconds := int(d.Seconds())
	if seconds < 1 {
		seconds = 1
	}
	return itoa(seconds)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	negative := n < 0
	if negative {
		n = -n
	}
	digits := []byte{}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	if negative {
		return "-" + string(digits)
	}
	return string(digits)
}
