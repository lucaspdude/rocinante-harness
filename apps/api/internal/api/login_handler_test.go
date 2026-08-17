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

// stubCmd is a CmdIface stand-in for tests. It writes preset data
// to its stdout and exits cleanly.
type stubCmd struct {
	outR  *io.PipeReader
	outW  *io.PipeWriter
	done  chan struct{}
}

func newStubCmd(payload string) *stubCmd {
	r, w := io.Pipe()
	c := &stubCmd{
		outR: r,
		outW: w,
		done: make(chan struct{}),
	}
	go func() {
		defer close(c.done)
		_, _ = io.WriteString(w, payload)
		_ = w.Close()
	}()
	return c
}

func (s *stubCmd) StdoutPipe() (io.ReadCloser, error) { return s.outR, nil }
func (s *stubCmd) Start() error                       { return nil }
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
	if len(body.Providers) < 2 {
		t.Fatalf("providers len = %d, want >= 2", len(body.Providers))
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

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		j := h.Jobs.Get(start.JobID)
		if j != nil {
			j.mu.Lock()
			state := j.state
			evs := len(j.events)
			j.mu.Unlock()
			if evs >= 1 && state != LoginJobRunning {
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	j := h.Jobs.Get(start.JobID)
	if j == nil {
		t.Fatal("job missing")
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	if len(j.events) < 1 {
		t.Fatalf("events = %d, want >= 1", len(j.events))
	}
	if j.events[0].Event != "ui_request" && j.events[0].Event != "spawn" {
		t.Errorf("first event = %q, want ui_request or spawn", j.events[0].Event)
	}
	if j.state != LoginJobComplete {
		t.Errorf("final state = %q, want complete", j.state)
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

	rr2 := httptest.NewRecorder()
	body := bytes.NewReader([]byte(`{"value":"abc"}`))
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/login/"+start.JobID+"/ack", body)
	r.ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusNoContent {
		t.Fatalf("ack status = %d, want 204, body=%s", rr2.Code, rr2.Body.String())
	}

	rr3 := httptest.NewRecorder()
	req3 := httptest.NewRequest(http.MethodGet, "/api/v1/login/"+start.JobID+"/status", nil)
	r.ServeHTTP(rr3, req3)
	if rr3.Code != http.StatusOK {
		t.Fatalf("status status = %d, want 200", rr3.Code)
	}
	var st LoginStatus
	_ = json.Unmarshal(rr3.Body.Bytes(), &st)
	if st.ProviderID != "openai" {
		t.Errorf("provider_id = %q", st.ProviderID)
	}
}

func TestLoginAckJobExpiredReturns410(t *testing.T) {
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

	// Force the job to expire via direct method call.
	job := h.Jobs.Get(start.JobID)
	if job == nil {
		t.Fatal("job missing")
	}
	job.Finish(LoginJobExpired, "test")

	rr2 := httptest.NewRecorder()
	body := bytes.NewReader([]byte(`{}`))
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
