// Package middleware contains HTTP middlewares for the api.
package middleware

import (
	"container/list"
	"net/http"
	"sync"
)

// IdempotencyCache is a simple LRU keyed by Idempotency-Key.
type IdempotencyCache struct {
	mu       sync.Mutex
	capacity int
	order    *list.List
	entries  map[string]*list.Element
}

// idempotencyEntry is the value held in the LRU.
type idempotencyEntry struct {
	key   string
	value []byte
}

// NewIdempotencyCache returns an LRU with the given capacity.
func NewIdempotencyCache(capacity int) *IdempotencyCache {
	if capacity <= 0 {
		capacity = 2048
	}
	return &IdempotencyCache{
		capacity: capacity,
		order:    list.New(),
		entries:  make(map[string]*list.Element, capacity),
	}
}

// Get returns the cached response body for the key, or nil if
// absent. Marks the entry as most-recently-used.
func (c *IdempotencyCache) Get(key string) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.entries[key]
	if !ok {
		return nil, false
	}
	c.order.MoveToFront(el)
	entry := el.Value.(*idempotencyEntry)
	return entry.value, true
}

// Put stores the value for the key. Evicts the oldest entry if
// the cache is full.
func (c *IdempotencyCache) Put(key string, value []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.entries[key]; ok {
		c.order.MoveToFront(el)
		el.Value.(*idempotencyEntry).value = value
		return
	}
	el := c.order.PushFront(&idempotencyEntry{key: key, value: value})
	c.entries[key] = el
	if c.order.Len() > c.capacity {
		oldest := c.order.Back()
		if oldest != nil {
			c.order.Remove(oldest)
			delete(c.entries, oldest.Value.(*idempotencyEntry).key)
		}
	}
}

// IdempotencyMiddleware short-circuits requests with a previously
// seen Idempotency-Key. The cache is best-effort (memory only).
func IdempotencyMiddleware(cache *IdempotencyCache) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := r.Header.Get("Idempotency-Key")
			if key == "" {
				next.ServeHTTP(w, r)
				return
			}
			if cached, ok := cache.Get(key); ok {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("X-Idempotency-Cache-State", "best-effort")
				w.Header().Set("X-Idempotency-Cache", "hit")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(cached)
				return
			}
			rw := &recordingWriter{ResponseWriter: w, body: &[]byte{}}
			rw.Header().Set("X-Idempotency-Cache-State", "best-effort")
			next.ServeHTTP(rw, r)
			if rw.status == http.StatusOK || rw.status == http.StatusCreated {
				cache.Put(key, *rw.body)
			}
		})
	}
}

// recordingWriter captures the response body so the idempotency
// middleware can cache it.
type recordingWriter struct {
	http.ResponseWriter
	body   *[]byte
	status int
}

func (r *recordingWriter) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *recordingWriter) Write(p []byte) (int, error) {
	*r.body = append(*r.body, p...)
	return r.ResponseWriter.Write(p)
}
