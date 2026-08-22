package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/lucaspdude/rocinante-harness/apps/api/internal/api/middleware"
	"github.com/lucaspdude/rocinante-harness/apps/api/internal/omp"
	sessionspkg "github.com/lucaspdude/rocinante-harness/apps/api/internal/sessions"
)

func writeScript(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "omp")
	script := "#!/usr/bin/env bash\n" + body + "\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatalf("write: %v", err)
	}
	return bin
}

func newTestRouter(t *testing.T, scriptBody string) (http.Handler, *omp.Manager) {
	t.Helper()
	bin := writeScript(t, scriptBody)
	manager := omp.NewManagerWithFactory(scriptFactory{bin: bin})
	mux := chi.NewRouter()
	mux.Post("/api/v1/sessions", CreateSessionHandler(manager, nil))
	mux.Get("/api/v1/sessions/{id}/events", StreamSessionHandler(manager))
	return mux, manager
}

type scriptFactory struct {
	bin string
}

func (f scriptFactory) NewSession(opts omp.Options) (*omp.Session, error) {
	opts.OpBin = f.bin
	return omp.Spawn(context.Background(), opts)
}

func TestSSE_FidelityOneToOne_V2(t *testing.T) {
	body := strings.Join([]string{
		`echo '{"protocol_version":2,"omp_version":"omp/17.3.4"}'`,
		`echo '{"type":"agent_start","seq":1}'`,
		`echo '{"type":"delta","seq":2,"text":"hello"}'`,
		`echo '{"type":"agent_end","seq":3}'`,
		`sleep 5`,
	}, "\n")
	h, _ := newTestRouter(t, body)
	srv := httptest.NewServer(h)
	defer srv.Close()

	created := doPostServer(t, srv, `{"omp_cwd":"/tmp"}`)
	id, _ := created["id"].(string)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/api/v1/sessions/"+id+"/events", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	buf := make([]byte, 4096)
	var captured strings.Builder
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			captured.Write(buf[:n])
		}
		if strings.Contains(captured.String(), "agent_end") {
			break
		}
		if err != nil {
			break
		}
	}

	cancel()
	got := captured.String()
	want := []string{
		"data: {\"type\":\"agent_start\",\"seq\":1}",
		"data: {\"type\":\"delta\",\"seq\":2,\"text\":\"hello\"}",
		"data: {\"type\":\"agent_end\",\"seq\":3}",
	}
	for _, w := range want {
		if !strings.Contains(got, w) {
			t.Errorf("response missing %q\nfull body:\n%s", w, got)
		}
	}
	if !strings.HasPrefix(got, "data: {") {
		t.Errorf("expected response to start with `data: {`, got: %q", firstChars(got, 80))
	}
}

func TestSSE_FidelityOneToOne_V1(t *testing.T) {
	body := strings.Join([]string{
		`echo '{"jsonrpc":"2.0","method":"ready"}'`,
		`echo '{"jsonrpc":"2.0","method":"frame","params":{"seq":1}}'`,
		`echo '{"jsonrpc":"2.0","method":"frame","params":{"seq":2,"text":"hi"}}'`,
		`sleep 5`,
	}, "\n")
	h, _ := newTestRouter(t, body)
	srv := httptest.NewServer(h)
	defer srv.Close()

	created := doPostServer(t, srv, `{"omp_cwd":"/tmp"}`)
	id, _ := created["id"].(string)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/api/v1/sessions/"+id+"/events", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	buf := make([]byte, 4096)
	var captured strings.Builder
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			captured.Write(buf[:n])
		}
		if strings.Count(captured.String(), "data: ") >= 2 {
			break
		}
		if err != nil {
			break
		}
	}
	cancel()
	got := captured.String()
	if !strings.Contains(got, `"params":{"seq":2,"text":"hi"}`) {
		t.Errorf("missing v1 frame; got:\n%s", got)
	}
}

func TestSSE_404OnMissingSession(t *testing.T) {
	h, _ := newTestRouter(t, `sleep 5`)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/inexistente/events", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "session_not_found") {
		t.Errorf("body = %s", rr.Body.String())
	}
}

func TestSSE_410OnClosedSession(t *testing.T) {
	body := strings.Join([]string{
		`echo '{"protocol_version":2,"omp_version":"omp/17.3.4"}'`,
		`echo '{"type":"agent_end","seq":1}'`,
		`sleep 5`,
	}, "\n")
	h, manager := newTestRouter(t, body)
	created := doPost(t, h, "/api/v1/sessions", `{"omp_cwd":"/tmp"}`)
	id, _ := created["id"].(string)
	if err := manager.Close(id); err != nil {
		t.Fatalf("close: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/"+id+"/events", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404 (closed session is gone)", rr.Code)
	}
}

func doPost(t *testing.T, h http.Handler, path, body string) map[string]any {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("POST %s: status %d, body %s", path, rr.Code, rr.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return out
}

func doPostServer(t *testing.T, srv *httptest.Server, body string) map[string]any {
	t.Helper()
	resp, err := http.Post(srv.URL+"/api/v1/sessions", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		buf, _ := io.ReadAll(resp.Body)
		t.Fatalf("POST status %d, body %s", resp.StatusCode, buf)
	}
	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return out
}

// TestPrompt_PersistsUserMessage verifies that wiring a JSONL store
// into PromptHandlerWithRecorder writes the user message into the
// log on prompt POST. The store is exercised end-to-end via the
// replay endpoint.
func TestPrompt_PersistsUserMessage(t *testing.T) {
	body := strings.Join([]string{
		`echo '{"protocol_version":2,"omp_version":"omp/17.3.4"}'`,
		`echo '{"type":"agent_start","seq":1}'`,
		`sleep 5`,
	}, "\n")
	bin := writeScript(t, body)
	manager := omp.NewManagerWithFactory(scriptFactory{bin: bin})
	shareDir := t.TempDir()
	store := sessionspkg.New(shareDir)
	mux := chi.NewRouter()
	mux.Post("/api/v1/sessions", CreateSessionHandler(manager, nil))
	mux.Post("/api/v1/sessions/{id}/prompt", WrapHandler(middleware.IdempotencyMiddleware(nil), PromptHandlerWithRecorder(manager, store)))
	mux.Get("/api/v1/sessions/{id}/messages", sessionspkg.ReplayHandler(store))

	srv := httptest.NewServer(mux)
	defer srv.Close()

	created := doPostServer(t, srv, `{"omp_cwd":"/tmp"}`)
	id, _ := created["id"].(string)

	// POST a prompt.
	promptBody := `{"text":"hello world"}`
	resp, err := http.Post(srv.URL+"/api/v1/sessions/"+id+"/prompt", "application/json", strings.NewReader(promptBody))
	if err != nil {
		t.Fatalf("prompt POST: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		buf, _ := io.ReadAll(resp.Body)
		t.Fatalf("prompt status = %d, body = %s", resp.StatusCode, buf)
	}

	// Replay should contain exactly one user message.
	entries, err := store.Replay(id, 0)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("replay entries = %d, want 1", len(entries))
	}
	if entries[0].Kind != "message" {
		t.Errorf("entry[0].kind = %q, want message", entries[0].Kind)
	}
	var msg map[string]any
	if err := json.Unmarshal(entries[0].Message, &msg); err != nil {
		t.Fatalf("unmarshal message: %v", err)
	}
	if msg["content"] != "hello world" {
		t.Errorf("content = %v, want hello world", msg["content"])
	}
	if msg["role"] != "user" {
		t.Errorf("role = %v, want user", msg["role"])
	}
}

func firstChars(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
