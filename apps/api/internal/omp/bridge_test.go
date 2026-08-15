package omp

import (
	"context"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func testdataPath(name string) string {
	return filepath.Join("testdata", name)
}

func TestSpawnV2Handshake(t *testing.T) {
	script := testdataPath("hello-ndjson-v2.sh")
	if _, err := os.Stat(script); err != nil {
		t.Skipf("testdata missing: %v", err)
	}
	// Wrap the script so it emits a v2 handshake line first.
	wrapper := filepath.Join(t.TempDir(), "omp")
	wrapperScript := `#!/usr/bin/env bash
echo '{"protocol_version":2,"omp_version":"omp/17.3.4","server":"omp"}'
sleep 1
`
	if err := os.WriteFile(wrapper, []byte(wrapperScript), 0o755); err != nil {
		t.Fatalf("write wrapper: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	sess, err := Spawn(ctx, Options{OpBin: wrapper, Cwd: t.TempDir()})
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	defer sess.Close()
	proto, ver := sess.Version()
	if proto != 2 {
		t.Errorf("ProtocolVersion = %d, want 2", proto)
	}
	if ver != "omp/17.3.4" {
		t.Errorf("OmpVersion = %q, want omp/17.3.4", ver)
	}
}

func TestSpawnV1Handshake(t *testing.T) {
	wrapper := filepath.Join(t.TempDir(), "omp")
	wrapperScript := `#!/usr/bin/env bash
echo '{"jsonrpc":"2.0","method":"ready"}'
sleep 1
`
	if err := os.WriteFile(wrapper, []byte(wrapperScript), 0o755); err != nil {
		t.Fatalf("write wrapper: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	sess, err := Spawn(ctx, Options{OpBin: wrapper, Cwd: t.TempDir()})
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	defer sess.Close()
	proto, _ := sess.Version()
	if proto != 1 {
		t.Errorf("ProtocolVersion = %d, want 1", proto)
	}
}

func TestClosePropagatesSIGTERM(t *testing.T) {
	wrapper := filepath.Join(t.TempDir(), "omp")
	wrapperScript := `#!/usr/bin/env bash
trap '' TERM
echo '{"protocol_version":2,"omp_version":"omp/17.3.4"}'
sleep 60
`
	if err := os.WriteFile(wrapper, []byte(wrapperScript), 0o755); err != nil {
		t.Fatalf("write wrapper: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	sess, err := Spawn(ctx, Options{OpBin: wrapper, Cwd: t.TempDir()})
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	pid := sess.Pid()
	if pid == 0 {
		t.Fatalf("pid = 0")
	}
	done := make(chan error, 1)
	go func() { done <- sess.Close() }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Errorf("Close did not return within 5s; SIGKILL escalation expected")
	}
	if processAlive(pid) {
		t.Errorf("process %d still alive after Close", pid)
	}
}

func processAlive(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	if err := proc.Signal(syscall.Signal(0)); err != nil {
		return false
	}
	return true
}
