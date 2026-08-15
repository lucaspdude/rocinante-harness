package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIdempotencyCacheLRU(t *testing.T) {
	c := NewIdempotencyCache(2)
	c.Put("a", []byte(`{"x":1}`))
	c.Put("b", []byte(`{"x":2}`))
	if v, ok := c.Get("a"); !ok || string(v) != `{"x":1}` {
		t.Errorf("Get a = %q, ok=%v", v, ok)
	}
	c.Put("c", []byte(`{"x":3}`))
	if _, ok := c.Get("b"); ok {
		t.Errorf("b should be evicted")
	}
	if v, ok := c.Get("a"); !ok || string(v) != `{"x":1}` {
		t.Errorf("Get a after eviction = %q, ok=%v", v, ok)
	}
	if v, ok := c.Get("c"); !ok || string(v) != `{"x":3}` {
		t.Errorf("Get c = %q, ok=%v", v, ok)
	}
}

func TestIdempotencyMiddlewareShortCircuit(t *testing.T) {
	c := NewIdempotencyCache(8)
	var calls int
	h := IdempotencyMiddleware(c)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))

	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodPost, "/x", nil)
		req.Header.Set("Idempotency-Key", "k1")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("status = %d", rr.Code)
		}
		if rr.Header().Get("X-Idempotency-Cache") != "hit" && i > 0 {
			t.Errorf("expected hit on retry, got %q", rr.Header().Get("X-Idempotency-Cache"))
		}
	}
	if calls != 1 {
		t.Errorf("calls = %d, want 1", calls)
	}
}

func TestIdempotencyMiddlewareNoKey(t *testing.T) {
	c := NewIdempotencyCache(8)
	var calls int
	h := IdempotencyMiddleware(c)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodPost, "/x", nil)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Body.String() != `{"ok":true}` {
			t.Errorf("body = %q", rr.Body.String())
		}
	}
	if calls != 3 {
		t.Errorf("calls = %d, want 3", calls)
	}
}

func TestIdempotencyMiddlewareBodyPropagates(t *testing.T) {
	c := NewIdempotencyCache(8)
	h := IdempotencyMiddleware(c)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf, _ := readAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(buf)
	}))
	req := httptest.NewRequest(http.MethodPost, "/x", bytes.NewReader([]byte(`{"y":1}`)))
	req.Header.Set("Idempotency-Key", "k2")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Body.String() != `{"y":1}` {
		t.Errorf("body = %q", rr.Body.String())
	}
}

func readAll(r interface{ Read(p []byte) (int, error) }) ([]byte, error) {
	var buf []byte
	tmp := make([]byte, 1024)
	for {
		n, err := r.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
		}
		if err != nil {
			if err.Error() == "EOF" {
				return buf, nil
			}
			return buf, err
		}
	}
}
