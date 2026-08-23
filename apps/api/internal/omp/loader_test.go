package omp

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Stub-binary pattern: drop a bash script in t.TempDir() that
// prints a v2 handshake line on stdout, then sleeps. The harness
// handshake parser expects { protocol_version, omp_version }
// followed by a newline; the stub must NOT write to stdin.

// stubOK prints a valid v2 handshake (omp/18.0.0) and stays
// alive so the probe can complete without the subprocess
// exiting mid-handshake. Mirrors handshake_test.go:56-71.
func stubOK(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "omp-ok")
	script := "#!/usr/bin/env bash\n" +
		"echo '{\"protocol_version\":2,\"omp_version\":\"omp/18.0.0\"}'\n" +
		"sleep 5\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatalf("write stub: %v", err)
	}
	return bin
}

// stubFail prints nothing to stdout and exits non-zero.
func stubFail(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "omp-fail")
	script := "#!/usr/bin/env bash\nexit 1\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatalf("write stub: %v", err)
	}
	return bin
}

// stubSlow prints a valid handshake but only after a 2 s sleep.
func stubSlow(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "omp-slow")
	script := "#!/usr/bin/env bash\n" +
		"sleep 2\n" +
		"echo '{\"protocol_version\":2,\"omp_version\":\"omp/18.0.0\"}'\n" +
		"sleep 5\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatalf("write stub: %v", err)
	}
	return bin
}

// 1. Start ctx cancel exits the goroutine within 1 s.
func TestDynamicLoader_StartCtxCancel(t *testing.T) {
	bin := stubOK(t)
	// probeBudget=200ms keeps the probe bounded; failThreshold=1
	// + 30ms failPause keeps the loop responsive.
	d := NewDynamicLoaderWithConfig(bin, 200*time.Millisecond, 1, 30*time.Millisecond, 30*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	d.Start(ctx)
	// Let the boot probe run.
	time.Sleep(50 * time.Millisecond)
	cancel()
	// After cancel, the in-flight probe is killed by
	// exec.CommandContext and Probe returns. The loop sees
	// ctx.Err() != nil and exits. Worst case ~250ms.
	// Call Close() to assert the goroutine has released
	// muStart (i.e. returned from the loop function).
	time.Sleep(500 * time.Millisecond)
	d.Close()
	d.muStart.Lock()
	started := d.started
	d.muStart.Unlock()
	if started {
		t.Errorf("after Close(), started = true; want false (loop did not exit)")
	}
}

// 2. Single-flight: 5 concurrent Rechecks return 1 success + 4
// ErrProbeInFlight.
func TestDynamicLoader_SingleFlight(t *testing.T) {
	bin := stubSlow(t) // 2 s sleep before handshake
	d := NewDynamicLoaderWithConfig(bin, 10*time.Second, 1, 50*time.Millisecond, 50*time.Millisecond)
	var wg sync.WaitGroup
	var inFlight, ok int32
	for range 5 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := d.Recheck(context.Background())
			if err == nil {
				atomic.AddInt32(&ok, 1)
			} else if err == ErrProbeInFlight {
				atomic.AddInt32(&inFlight, 1)
			}
		}()
	}
	wg.Wait()
	if inFlight+ok != 5 {
		t.Errorf("inFlight(%d) + ok(%d) != 5", inFlight, ok)
	}
	if ok != 1 {
		t.Errorf("ok = %d, want 1", ok)
	}
	if inFlight != 4 {
		t.Errorf("inFlight = %d, want 4", inFlight)
	}
}

// 3. Five failures trigger failPause. We disable the
// background loop (no Start) and call Probe 6 times directly:
// the first 5 should fail, the 6th should still be allowed
// (Probe doesn't gate on consecutiveFailures; only loop
// respects the backoff). To assert the loop backoff, we use
// Start + a stub that always fails, and assert within a tight
func TestDynamicLoader_FiveFailuresPause(t *testing.T) {
	bin := stubFail(t)
	// failThreshold=5, failPause=300ms, probeInterval=20ms.
	// Budget per probe = 200ms. We use a longer total window
	// so the backoff can cycle and we see at least 5 failures
	// AND the 6th being paused.
	d := NewDynamicLoaderWithConfig(bin, 100*time.Millisecond, 5, 500*time.Millisecond, 20*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	d.Start(ctx)
	// Wait long enough for 5 probes + the 500ms pause to fire.
	time.Sleep(1500 * time.Millisecond)
	cancel()
	time.Sleep(300 * time.Millisecond) // let the in-flight probe end
	d.Close()
	d.mu.RLock()
	cf := d.consecutiveFailures
	d.mu.RUnlock()
	if cf < 5 {
		t.Errorf("consecutiveFailures = %d, want >= 5", cf)
	}
	// The pause AFTER the 5th failure means we expect roughly
	// 5-6 failures in the 1.5s window. We don't assert an
	// exact ceiling here because the probe budget adds jitter;
	// the key contract is "5 failures triggered the backoff".
	if cf > 7 {
		t.Errorf("consecutiveFailures = %d, backoff not triggered", cf)
	}
}

// 4. OmpVersion race: 100 readers + 1 writer, no race detected.
func TestDynamicLoader_OmpVersionConcurrency(t *testing.T) {
	bin := stubOK(t)
	d := NewDynamicLoader(bin)
	// Seed a known version so readers see something.
	d.mu.Lock()
	d.proto = 2
	d.version = "omp/18.0.0"
	d.mu.Unlock()
	var wg sync.WaitGroup
	for range 100 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 50 {
				_, _ = d.OmpVersion()
			}
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for j := range 50 {
			d.mu.Lock()
			d.proto = (j % 2) + 1
			d.version = "omp/test"
			d.mu.Unlock()
		}
	}()
	wg.Wait()
}

// 5. Recheck returns ErrProbeInFlight when probeMu is held by
// another goroutine; releases cleanly.
func TestDynamicLoader_RecheckInFlightNoDeadlock(t *testing.T) {
	bin := stubOK(t)
	d := NewDynamicLoaderWithConfig(bin, 5*time.Second, 1, 50*time.Millisecond, 50*time.Millisecond)
	// Manually hold probeMu to simulate a probe in flight.
	if !d.probeMu.TryLock() {
		t.Skip("could not acquire probeMu to simulate in-flight probe")
	}
	// Recheck should return ErrProbeInFlight immediately.
	err := d.Recheck(context.Background())
	if err != ErrProbeInFlight {
		t.Errorf("Recheck = %v, want ErrProbeInFlight", err)
	}
	d.probeMu.Unlock()
	// After release, a fresh Recheck can acquire probeMu.
	// (We don't care about its return value here — the contract
	// under test is "no deadlock".)
	_ = d.Recheck(context.Background())
}
