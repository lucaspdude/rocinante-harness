package auth

import (
	"crypto/ed25519"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAuthMiddlewareMissing(t *testing.T) {
	pk, sk, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	loader := &StaticKeyLoader{Pk: pk}
	called := false
	h := AuthMiddleware(loader, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	_ = sk
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "auth_missing") {
		t.Errorf("body = %s", rr.Body.String())
	}
	if called {
		t.Errorf("downstream should not be called")
	}
}

func TestAuthMiddlewareWrongScheme(t *testing.T) {
	pk, _, _ := ed25519.GenerateKey(nil)
	h := AuthMiddleware(&StaticKeyLoader{Pk: pk}, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Authorization", "Basic abc")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rr.Code)
	}
}

func TestAuthMiddlewareBearerValid(t *testing.T) {
	_, sk, _ := ed25519.GenerateKey(nil)
	pk := sk.Public().(ed25519.PublicKey)
	signer := NewSigner(sk, pk, "test-pass")
	access, err := signer.IssueAccess("dev-123")
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	called := false
	h := AuthMiddleware(&StaticKeyLoader{Pk: pk}, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if DeviceIDFromContext(r.Context()) != "dev-123" {
			t.Errorf("device id missing in context")
		}
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Authorization", "Bearer "+access)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
	if !called {
		t.Errorf("downstream should be called")
	}
}

func TestAuthMiddlewareBearerInvalid(t *testing.T) {
	pk, _, _ := ed25519.GenerateKey(nil)
	h := AuthMiddleware(&StaticKeyLoader{Pk: pk}, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Authorization", "Bearer not-a-token")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "auth_invalid_token") {
		t.Errorf("body = %s", rr.Body.String())
	}
}
