package runner

import (
	"path/filepath"
	"testing"
)

func TestStateRoundTrip(t *testing.T) {
	dir := t.TempDir()
	r := New(filepath.Join(dir, "cache"), filepath.Join(dir, "logs"))
	r.state.APIPID = 1234
	r.state.WebPID = 5678
	r.state.APIPort = 30179
	r.state.WebPort = 30178
	if err := r.PIDFile(); err != nil {
		t.Fatalf("pid file: %v", err)
	}
	got, err := LoadState(filepath.Join(dir, "cache"))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.APIPID != 1234 || got.WebPID != 5678 {
		t.Errorf("state mismatch: %+v", got)
	}
}

func TestTailLogsEmpty(t *testing.T) {
	dir := t.TempDir()
	out, err := TailLogs(dir, 10)
	if err != nil {
		t.Fatalf("tail: %v", err)
	}
	if out == "" {
		t.Errorf("expected default output")
	}
}
