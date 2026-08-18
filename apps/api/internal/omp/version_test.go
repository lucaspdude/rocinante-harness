package omp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type stubLoader struct {
	bin   string
	proto int
	ver   string
}

func (s stubLoader) OmpBin() string             { return s.bin }
func (s stubLoader) OmpVersion() (int, string) { return s.proto, s.ver }

func TestMetaHandlerOmpFound(t *testing.T) {
	providers := []MetaProviderInfo{
		{ID: "anthropic", SupportsLogin: true, Authenticated: true, EnvVars: []string{"ANTHROPIC_API_KEY"}},
		{ID: "openai", SupportsLogin: true, Authenticated: false, EnvVars: []string{"OPENAI_API_KEY"}},
		{ID: "minimax", SupportsLogin: true, Authenticated: true, EnvVars: []string{"MINIMAX_API_KEY"}},
	}
	h := NewMetaHandler(
		stubLoader{bin: "/usr/local/bin/omp", proto: 2, ver: "omp/17.3.4"},
		"0.1.0",
		providers,
	)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/meta", nil)
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	var body MetaResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v body=%s", err, rr.Body.String())
	}
	if body.APIVersion != "0.1.0" {
		t.Errorf("APIVersion = %q, want 0.1.0", body.APIVersion)
	}
	if body.OmpVersion != "omp/17.3.4" {
		t.Errorf("OmpVersion = %q", body.OmpVersion)
	}
	if body.ProtocolVersion != 2 {
		t.Errorf("ProtocolVersion = %d", body.ProtocolVersion)
	}
	if body.OmpBin != "/usr/local/bin/omp" {
		t.Errorf("OmpBin = %q", body.OmpBin)
	}
	if len(body.Providers) != 3 {
		t.Fatalf("Providers len = %d, want 3", len(body.Providers))
	}
	if !body.Providers[0].Authenticated {
		t.Errorf("Providers[0] (anthropic) Authenticated = false, want true")
	}
	if body.Providers[1].Authenticated {
		t.Errorf("Providers[1] (openai) Authenticated = true, want false")
	}
	if len(body.Providers[0].EnvVars) != 1 || body.Providers[0].EnvVars[0] != "ANTHROPIC_API_KEY" {
		t.Errorf("Providers[0].EnvVars = %+v", body.Providers[0].EnvVars)
	}
}

func TestMetaHandlerOmpMissing(t *testing.T) {
	h := NewMetaHandler(stubLoader{bin: ""}, "0.1.0", nil)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/meta", nil)
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "omp_not_found") {
		t.Errorf("body = %q, want code omp_not_found", rr.Body.String())
	}
}
