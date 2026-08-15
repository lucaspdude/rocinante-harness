package omp

import (
	"testing"
)

func TestParseHandshakeV2(t *testing.T) {
	line := `{"protocol_version":2,"omp_version":"omp/17.3.4","server":"omp"}`
	hs, err := parseHandshake(line)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if hs.ProtocolVersion != 2 {
		t.Errorf("ProtocolVersion = %d, want 2", hs.ProtocolVersion)
	}
	if hs.OmpVersion != "omp/17.3.4" {
		t.Errorf("OmpVersion = %q, want omp/17.3.4", hs.OmpVersion)
	}
}

func TestParseHandshakeV1(t *testing.T) {
	line := `{"jsonrpc":"2.0","method":"ready"}`
	hs, err := parseHandshake(line)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if hs.ProtocolVersion != 1 {
		t.Errorf("ProtocolVersion = %d, want 1", hs.ProtocolVersion)
	}
	if hs.OmpVersion != "" {
		t.Errorf("OmpVersion = %q, want empty for v1 without omp_version", hs.OmpVersion)
	}
}

func TestParseHandshakeInvalid(t *testing.T) {
	_, err := parseHandshake(`not json`)
	if err == nil {
		t.Errorf("parse: expected error for non-json")
	}
}

func TestParseHandshakeUnknown(t *testing.T) {
	_, err := parseHandshake(`{"foo":1}`)
	if err == nil {
		t.Errorf("parse: expected error for missing protocol_version and jsonrpc")
	}
}
