package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
)

type stubCmd struct {
	stdoutR  *io.PipeReader
	stdoutW  *io.PipeWriter
	stdinBuf *bytes.Buffer
	done     chan struct{}
}

func newStubCmd(payload string) *stubCmd {
	r, w := io.Pipe()
	c := &stubCmd{
		stdoutR:  r,
		stdoutW:  w,
		stdinBuf: &bytes.Buffer{},
		done:     make(chan struct{}),
	}
	go func() {
		defer close(c.done)
		_, _ = io.WriteString(w, payload)
		_ = w.Close()
	}()
	return c
}

// stubStdin wraps bytes.Buffer as io.WriteCloser. Pointer
// receiver so it satisfies io.Closer.
type stubStdin struct{ *bytes.Buffer }

func (s *stubStdin) Close() error { return nil }

func (s *stubCmd) StdinPipe() (io.WriteCloser, error) {
	return &stubStdin{s.stdinBuf}, nil
}
func (s *stubCmd) StdoutPipe() (io.ReadCloser, error) { return s.stdoutR, nil }
func (s *stubCmd) StderrPipe() (io.ReadCloser, error) { return s.stdoutR, nil }
func (s *stubCmd) Start() error                        { return nil }
func (s *stubCmd) Wait() error                        { <-s.done; return nil }

type testProbe map[string]bool

func (t testProbe) IsConfigured(name string) bool { return t[name] }

func newTestHandlers() *LoginHandlers {
	probe := testProbe{
		"anthropic": true,
		"openai":    false,
	}
	cache := NewLoginProvidersCache(staticLoginProviders{probe: probe})
	return &LoginHandlers{
		Providers: cache,
		Jobs:      NewLoginJobs(),
		CmdFactory: func(_ context.Context, _ string, _ []string) CmdIface {
			return newStubCmd(`{"method":"select","title":"Pick","options":["foo","bar"]}` + "\n" + "ok\n")
		},
	}
}

func mount(h *LoginHandlers) http.Handler {
	r := chi.NewRouter()
	r.Get("/api/v1/login/providers", h.LoginProvidersHandler)
	r.Post("/api/v1/login/start/{provider}", h.LoginStartHandler)
	r.Get("/api/v1/login/{jobId}/stream", h.LoginStreamHandler)
	r.Post("/api/v1/login/{jobId}/ack", h.LoginAckHandler)
	r.Get("/api/v1/login/{jobId}/status", h.LoginStatusHandler)
	return r
}

func TestProvidersCapabilitiesShape(t *testing.T) {
	h := newTestHandlers()
	r := mount(h)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/login/providers", nil)
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	var body LoginProvidersResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Providers) < 2 {
		t.Fatalf("providers len = %d, want >= 2", len(body.Providers))
	}
	for _, p := range body.Providers {
		if p.ID == "anthropic" && !p.Authenticated {
			t.Error("anthropic should be configured per testProbe")
		}
		if p.ID == "openai" && p.Authenticated {
			t.Error("openai should not be configured per testProbe")
		}
		if !p.SupportsLogin {
			t.Errorf("provider %q missing supports_login=true", p.ID)
		}
		if len(p.EnvVars) == 0 {
			t.Errorf("provider %q has empty env_vars", p.ID)
		}
	}
}

func TestLoginProvidersEndpoint(t *testing.T) {
	h := newTestHandlers()
	r := mount(h)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/login/providers", nil)
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	var body LoginProvidersResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	var anthropic *LoginProviderInfo
	for i, p := range body.Providers {
		if p.ID == "anthropic" {
			anthropic = &body.Providers[i]
		}
	}
	if anthropic == nil {
		t.Fatal("anthropic not in list")
	}
	if !anthropic.Authenticated {
		t.Error("anthropic Authenticated = false, want true")
	}
}

func TestLoginStartReturns202(t *testing.T) {
	h := newTestHandlers()
	r := mount(h)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/login/start/anthropic", nil)
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202, body=%s", rr.Code, rr.Body.String())
	}
	var body LoginStartResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.JobID == "" {
		t.Error("job_id empty")
	}
	if body.ProviderID != "anthropic" {
		t.Errorf("provider_id = %q", body.ProviderID)
	}
	if !strings.HasPrefix(body.StreamURL, "/api/v1/login/") {
		t.Errorf("stream_url = %q", body.StreamURL)
	}
}

func waitForState(t *testing.T, h *LoginHandlers, id string, timeout time.Duration, pred func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if pred() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestLoginStreamEmitsEvents(t *testing.T) {
	h := newTestHandlers()
	r := mount(h)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/login/start/anthropic", nil)
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusAccepted {
		t.Fatalf("start status = %d, want 202", rr.Code)
	}
	var start LoginStartResponse
	_ = json.Unmarshal(rr.Body.Bytes(), &start)

	completed := false
	waitForState(t, h, start.JobID, 2*time.Second, func() bool {
		job := h.Jobs.Get(start.JobID)
		if job == nil {
			return false
		}
		job.mu.Lock()
		state := job.state
		evs := len(job.events)
		job.mu.Unlock()
		if evs >= 1 && state != LoginJobRunning {
			completed = true
			return true
		}
		return false
	})
	if !completed {
		t.Fatal("job did not complete within timeout")
	}
	job := h.Jobs.Get(start.JobID)
	if job == nil {
		t.Fatal("job missing")
	}
	job.mu.Lock()
	final := job.state
	events := append([]LoginEvent(nil), job.events...)
	job.mu.Unlock()
	if final != LoginJobComplete {
		t.Errorf("final state = %q, want complete", final)
	}
	if len(events) == 0 {
		t.Fatal("no events emitted")
	}
}

func TestLoginAckRoundtrip(t *testing.T) {
	h := newTestHandlers()
	r := mount(h)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/login/start/openai", nil)
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusAccepted {
		t.Fatalf("start status = %d, want 202", rr.Code)
	}
	var start LoginStartResponse
	_ = json.Unmarshal(rr.Body.Bytes(), &start)

	body := strings.NewReader(`{"value":"abc"}`)
	rr2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/login/"+start.JobID+"/ack", body)
	r.ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", rr2.Code)
	}

	rr3 := httptest.NewRecorder()
	req3 := httptest.NewRequest(http.MethodGet, "/api/v1/login/"+start.JobID+"/status", nil)
	r.ServeHTTP(rr3, req3)
	if rr3.Code != http.StatusOK {
		t.Errorf("status status = %d, want 200", rr3.Code)
	}
	var st LoginStatus
	_ = json.Unmarshal(rr3.Body.Bytes(), &st)
	if st.JobID == "" {
		t.Error("JobID empty in status")
	}
}

func TestLoginAckJobExpiredReturns410(t *testing.T) {
	h := newTestHandlers()
	r := mount(h)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/login/start/anthropic", nil)
	r.ServeHTTP(rr, req)
	var start LoginStartResponse
	_ = json.Unmarshal(rr.Body.Bytes(), &start)

	job := h.Jobs.Get(start.JobID)
	if job == nil {
		t.Fatal("job missing")
	}
	job.Finish(LoginJobExpired, "test")

	rr2 := httptest.NewRecorder()
	body := strings.NewReader(`{}`)
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/login/"+start.JobID+"/ack", body)
	r.ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusGone {
		t.Errorf("status = %d, want 410", rr2.Code)
	}
}

func TestJobIDGeneration(t *testing.T) {
	seen := make(map[string]struct{})
	for range 100 {
		id := newLoginJobID()
		if _, dup := seen[id]; dup {
			t.Fatalf("duplicate id %q", id)
		}
		if !strings.HasPrefix(id, "lj_") {
			t.Fatalf("id %q missing prefix", id)
		}
		seen[id] = struct{}{}
	}
}
