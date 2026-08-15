package omp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type stubLoader struct {
	bin    string
	proto  int
	ver    string
}

func (s stubLoader) OmpBin() string { return s.bin }
func (s stubLoader) OmpVersion() (int, string) { return s.proto, s.ver }

func TestMetaHandlerOmpFound(t *testing.T) {
	h := NewMetaHandler(stubLoader{bin: "/usr/local/bin/omp", proto: 2, ver: "omp/17.3.4"}, "0.1.0")
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
}

func TestMetaHandlerOmpMissing(t *testing.T) {
	h := NewMetaHandler(stubLoader{bin: ""}, "0.1.0")
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
