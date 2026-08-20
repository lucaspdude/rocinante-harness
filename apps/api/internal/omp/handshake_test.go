package omp

import (
	"context"
	"os"
	"path/filepath"
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

// The real omp binary is ~178MB and takes ~750ms just to print its
// version on modest hardware. A fallback budget tighter than that
// silently yields an empty omp_version, which surfaces as "omp ?"
// in the StatusPill even though the handshake itself succeeded.
func TestFallbackOmpVersionToleratesSlowStart(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "omp-slow")
	script := "#!/usr/bin/env bash\nsleep 0.75\necho 'omp/17.3.7'\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatalf("write stub: %v", err)
	}

	got, err := fallbackOmpVersion(context.Background(), bin)
	if err != nil {
		t.Fatalf("fallbackOmpVersion: %v", err)
	}
	if got != "omp/17.3.7" {
		t.Errorf("fallbackOmpVersion = %q, want omp/17.3.7", got)
	}
}

func TestFallbackOmpVersionPrefixesBareVersion(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "omp-bare")
	if err := os.WriteFile(bin, []byte("#!/usr/bin/env bash\necho '17.3.7'\n"), 0o755); err != nil {
		t.Fatalf("write stub: %v", err)
	}

	got, err := fallbackOmpVersion(context.Background(), bin)
	if err != nil {
		t.Fatalf("fallbackOmpVersion: %v", err)
	}
	if got != "omp/17.3.7" {
		t.Errorf("fallbackOmpVersion = %q, want omp/17.3.7", got)
	}
}
