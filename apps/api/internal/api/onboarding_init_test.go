package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

// newOnboardingRouter builds a chi router with the minimum
// setup needed to exercise OnboardingInit in isolation. We
// don't need auth or the keystore for the init flow.
func newOnboardingRouter(shareDir string) http.Handler {
	r := chi.NewRouter()
	r.Post("/api/v1/onboarding/init", OnboardingInit(shareDir))
	return r
}

func TestOnboardingInitSuccess(t *testing.T) {
	dir := t.TempDir()
	r := newOnboardingRouter(dir)

	body, _ := json.Marshal(map[string]string{
		"passphrase": "supersecret1234",
		"locale":     "en-US",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/onboarding/init", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rr.Code, rr.Body.String())
	}

	// .ed25519 and .ed25519.bak should exist on disk.
	for _, name := range []string{".ed25519", ".ed25519.bak"} {
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("%s missing: %v", name, err)
		}
		// The key file must not be world-readable.
		info, _ := os.Stat(path)
		if mode := info.Mode().Perm(); mode&0o077 != 0 {
			t.Fatalf("%s mode = %o, expected no world/group perms", name, mode)
		}
	}
}

func TestOnboardingInitRejectsShortPassphrase(t *testing.T) {
	dir := t.TempDir()
	r := newOnboardingRouter(dir)

	body, _ := json.Marshal(map[string]string{
		"passphrase": "short",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/onboarding/init", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "passphrase_too_short") {
		t.Errorf("body = %q, want code passphrase_too_short", rr.Body.String())
	}
}

func TestOnboardingInitRefusesReinit(t *testing.T) {
	dir := t.TempDir()
	// Pre-create .ed25519 so the handler sees the api is
	// already initialized.
	if err := os.WriteFile(filepath.Join(dir, ".ed25519"), []byte("existing"), 0o600); err != nil {
		t.Fatal(err)
	}
	r := newOnboardingRouter(dir)

	body, _ := json.Marshal(map[string]string{
		"passphrase": "supersecret1234",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/onboarding/init", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409; body = %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "already_initialized") {
		t.Errorf("body = %q, want code already_initialized", rr.Body.String())
	}
}

func TestOnboardingInitBadJSON(t *testing.T) {
	dir := t.TempDir()
	r := newOnboardingRouter(dir)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/onboarding/init", bytes.NewReader([]byte("not json")))
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
}
