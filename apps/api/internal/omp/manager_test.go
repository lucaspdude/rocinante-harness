package omp

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

type stubSession struct {
	proto    int
	version  string
	closed   bool
	closeMu  sync.Mutex
}

func (s *stubSession) Close() error {
	s.closeMu.Lock()
	defer s.closeMu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	return nil
}

type stubFactory struct {
	mu      sync.Mutex
	created int
}

func (f *stubFactory) NewSession(opts Options) (*Session, error) {
	f.mu.Lock()
	f.created++
	f.mu.Unlock()
	// Return nil here; stubSession is illustrative only.
	_ = opts
	return nil, nil
}

func TestManagerCreateAndGet(t *testing.T) {
	m := NewManagerWithFactory(&stubFactory{})
	// Stub NewSession returns nil; we need a working session for
	// the manager to keep. Use a factory that returns a real
	// Session backed by a stub script.
	m2 := NewManagerWithFactory(spawnFactoryStub{t: t})
	rec, err := m2.Create(t.TempDir())
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if rec.ID == "" {
		t.Errorf("id empty")
	}
	if rec.State != "running" {
		t.Errorf("state = %q, want running", rec.State)
	}
	if rec.ProtocolVersion != 2 {
		t.Errorf("protocol = %d, want 2", rec.ProtocolVersion)
	}
	if m2.Get(rec.ID) == nil {
		t.Errorf("Get returned nil for live session")
	}
	m2.CloseAll()
	_ = m
}

func TestManagerCloseRemoves(t *testing.T) {
	m := NewManagerWithFactory(spawnFactoryStub{t: t})
	rec, err := m.Create(t.TempDir())
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := m.Close(rec.ID); err != nil {
		t.Errorf("close: %v", err)
	}
	if m.Get(rec.ID) != nil {
		t.Errorf("Get returned non-nil after close")
	}
}

func TestManagerCloseAll(t *testing.T) {
	m := NewManagerWithFactory(spawnFactoryStub{t: t})
	for i := 0; i < 5; i++ {
		if _, err := m.Create(t.TempDir()); err != nil {
			t.Fatalf("create %d: %v", i, err)
		}
	}
	if m.Count() != 5 {
		t.Errorf("count = %d, want 5", m.Count())
	}
	m.CloseAll()
	if m.Count() != 0 {
		t.Errorf("count after CloseAll = %d, want 0", m.Count())
	}
}

func TestManagerMaxConcurrent(t *testing.T) {
	m := NewManagerWithFactory(spawnFactoryStub{t: t})
	for i := 0; i < MaxSessionsConcurrent; i++ {
		if _, err := m.Create(t.TempDir()); err != nil {
			t.Fatalf("create %d: %v", i, err)
		}
	}
	if _, err := m.Create(t.TempDir()); err == nil {
		t.Errorf("expected max sessions error")
	}
	m.CloseAll()
}

func TestManagerRaceAcrossGoroutines(t *testing.T) {
	m := NewManagerWithFactory(spawnFactoryStub{t: t})
	var wg sync.WaitGroup
	for range 32 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rec, err := m.Create(t.TempDir())
			if err != nil {
				return
			}
			_ = m.Get(rec.ID)
			_ = m.Close(rec.ID)
		}()
	}
	wg.Wait()
	if m.Count() != 0 {
		t.Errorf("count = %d, want 0", m.Count())
	}
}

func TestSpawnV2HandshakeFromScript(t *testing.T) {
	wrapper := filepath.Join(t.TempDir(), "omp")
	script := `#!/usr/bin/env bash
echo '{"protocol_version":2,"omp_version":"omp/17.3.4"}'
sleep 1
`
	if err := os.WriteFile(wrapper, []byte(script), 0o755); err != nil {
		t.Fatalf("write: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	sess, err := Spawn(ctx, Options{OpBin: wrapper})
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	defer sess.Close()
	proto, ver := sess.Version()
	if proto != 2 || ver != "omp/17.3.4" {
		t.Errorf("Version = (%d, %q), want (2, omp/17.3.4)", proto, ver)
	}
}

// spawnFactoryStub uses a script that emits a v2 handshake and
// sleeps. Manager tests use it to get real, working sessions.
type spawnFactoryStub struct {
	t *testing.T
}

func (f spawnFactoryStub) NewSession(opts Options) (*Session, error) {
	wrapper := filepath.Join(f.t.TempDir(), "omp")
	script := `#!/usr/bin/env bash
echo '{"protocol_version":2,"omp_version":"omp/17.3.4"}'
sleep 30
`
	if err := os.WriteFile(wrapper, []byte(script), 0o755); err != nil {
		return nil, err
	}
	opts.OpBin = wrapper
	return Spawn(context.Background(), opts)
}
