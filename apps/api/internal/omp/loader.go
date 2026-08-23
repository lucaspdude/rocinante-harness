package omp

import (
	"bufio"
	"context"
	"errors"
	"log"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

// Phase 7 — item 02: DynamicLoader.
//
// Replaces the static `staticMetaLoader` from cmd/api/main.go that
// froze the omp version on a single boot-time probe. A clean
// install of the harness takes ~6 s to start the 178 MB omp
// binary; the boot probe's 5 s handshake budget was lost
// silently and the empty version persisted for the lifetime of
// the api process.
//
// The loader spawns a background probe loop (every 30 s) AND
// exposes an HTTP-driven Recheck trigger. A single-flight guard
// bounds the worst case to one active probe subprocess at a time,
// even under concurrent Recheck calls. Failures back off after
// 5 consecutive misses (5 min pause), then resume.

var errProbeInFlight = errors.New("probe in flight")

// ErrProbeInFlight is the exported sentinel so the meta refresh
// handler (in version.go) can treat it as a no-op success.
var ErrProbeInFlight = errProbeInFlight

// Production timings.
const (
	defaultProbeBudget   = 15 * time.Second
	defaultFailThreshold = 5
	defaultFailPause     = 5 * time.Minute
	defaultProbeInterval = 30 * time.Second
)

// DynamicLoader holds the cached omp metadata plus the
// single-flight + subprocess state needed to re-probe safely.
type DynamicLoader struct {
	bin string

	mu        sync.RWMutex
	proto     int
	version   string
	errMsg    string
	lastProbe time.Time

	probeMu sync.Mutex
	cmdMu   sync.Mutex
	cmd     *exec.Cmd

	probeBudget   time.Duration
	failThreshold int
	failPause     time.Duration
	probeInterval time.Duration

	started bool
	muStart sync.Mutex
	cancel  context.CancelFunc

	// consecutiveFailures is read by Recheck; written by Probe.
	consecutiveFailures int
}

// NewDynamicLoader returns a loader with production defaults.
func NewDynamicLoader(bin string) *DynamicLoader {
	return &DynamicLoader{
		bin:           bin,
		probeBudget:   defaultProbeBudget,
		failThreshold: defaultFailThreshold,
		failPause:     defaultFailPause,
		probeInterval: defaultProbeInterval,
	}
}

// NewDynamicLoaderWithConfig returns a loader with explicit timing
// config (used by tests that need fast cycles).
func NewDynamicLoaderWithConfig(
	bin string,
	probeBudget time.Duration,
	failThreshold int,
	failPause time.Duration,
	probeInterval time.Duration,
) *DynamicLoader {
	return &DynamicLoader{
		bin:           bin,
		probeBudget:   probeBudget,
		failThreshold: failThreshold,
		failPause:     failPause,
		probeInterval: probeInterval,
	}
}

func (d *DynamicLoader) OmpBin() string { return d.bin }

func (d *DynamicLoader) OmpVersion() (int, string) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.proto, d.version
}

// LastError returns the most recent probe failure message. Used
// for logging in the meta handler. Returns "" on success.
func (d *DynamicLoader) LastError() string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.errMsg
}

// Probe runs synchronously; populates proto/version/errMsg and
// (transiently) cmd. Safe under concurrent calls — second caller
// gets ErrProbeInFlight. Returns nil on success, an error on
// failure.
func (d *DynamicLoader) Probe(ctx context.Context) error {
	// Single-flight guard: only one probe at a time.
	if !d.probeMu.TryLock() {
		return ErrProbeInFlight
	}
	defer d.probeMu.Unlock()

	probeCtx, cancel := context.WithTimeout(ctx, d.probeBudget)
	defer cancel()

	// Spawn the subprocess. We track it via cmdMu so Close()
	// can SIGTERM an in-flight probe.
	cmd := exec.CommandContext(probeCtx, d.bin, "--mode", "rpc-ui")
	d.cmdMu.Lock()
	d.cmd = cmd
	d.cmdMu.Unlock()

	// Always clear the cmd field on exit.
	defer func() {
		d.cmdMu.Lock()
		if d.cmd == cmd {
			d.cmd = nil
		}
		// Defensive: if the subprocess is somehow still alive
		// after the context expired, SIGTERM it. The exec.Cmd
		// contract: Process is set after Start; if Start failed
		// we have no Process to signal.
		if cmd.Process != nil {
			_ = cmd.Process.Signal(syscall.SIGTERM)
		}
		d.cmdMu.Unlock()
	}()

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return d.recordFailure("stdin pipe: " + err.Error())
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return d.recordFailure("stdout pipe: " + err.Error())
	}
	if _, err := cmd.StderrPipe(); err != nil {
		return d.recordFailure("stderr pipe: " + err.Error())
	}
	if err := cmd.Start(); err != nil {
		return d.recordFailure("omp start: " + err.Error())
	}
	// stdin is intentionally never written; the handshake must
	// come from omp's stdout.
	_ = stdin

	br := bufio.NewReader(stdout)
	hs, _, err := readHandshakeWithLeftover(probeCtx, br)
	if err != nil {
		_ = cmd.Process.Signal(syscall.SIGTERM)
		_, _ = cmd.Process.Wait()
		// Phase 7.5 item B: omp 18.x refuses to emit a handshake
		// until the user has at least one provider key
		// configured (it prints "No models available" and
		// closes stdin, which arrives here as EOF). The cheap
		// `omp --version` probe still works in that state and
		// is enough to populate the version field; the meta
		// handler keeps reporting 503 omp_version_unknown
		// for the *providers* payload, but the status pill
		// and version string become accurate. Recover via the
		// fallback so the loader stops returning a frozen
		// empty version after the first install.
		if v, ferr := fallbackOmpVersion(probeCtx, d.bin); ferr == nil {
			d.mu.Lock()
			d.proto = 0
			d.version = v
			d.errMsg = ""
			d.lastProbe = time.Now()
			d.consecutiveFailures = 0
			d.mu.Unlock()
			return nil
		}
		return d.recordFailure("handshake: " + err.Error())
	}
	// Hand-off: detach the live session. From this point Probe
	// is done — the session is the caller's responsibility (we
	// close it here to free the subprocess; a successful probe
	// only needs the handshake data, not the live session).
	_ = cmd.Process.Signal(syscall.SIGTERM)
	_, _ = cmd.Process.Wait()

	ompVersion := hs.OmpVersion
	if ompVersion == "" {
		fallback, ferr := fallbackOmpVersion(probeCtx, d.bin)
		if ferr == nil {
			ompVersion = fallback
		}
	}

	d.mu.Lock()
	d.proto = hs.ProtocolVersion
	d.version = ompVersion
	d.errMsg = ""
	d.lastProbe = time.Now()
	d.consecutiveFailures = 0
	d.mu.Unlock()
	return nil
}

// recordFailure updates the loader's failure state and returns
// the formatted error.
func (d *DynamicLoader) recordFailure(reason string) error {
	d.mu.Lock()
	d.errMsg = reason
	d.lastProbe = time.Now()
	d.consecutiveFailures++
	cf := d.consecutiveFailures
	d.mu.Unlock()
	log.Printf("api: omp probe failed: %s; consecutive_failures=%d", reason, cf)
	return errors.New(reason)
}
// Start launches the background probe loop. Idempotent. Honors
// ctx for both per-probe deadline and inter-probe wait.
func (d *DynamicLoader) Start(ctx context.Context) {
	d.muStart.Lock()
	defer d.muStart.Unlock()
	if d.started {
		return
	}
	d.started = true
	bgCtx, cancel := context.WithCancel(ctx)
	d.cancel = cancel
	go d.loop(bgCtx)
}

func (d *DynamicLoader) loop(ctx context.Context) {
	for {
		if ctx.Err() != nil {
			return
		}
		// Best-effort initial probe at boot.
		probeCtx, cancel := context.WithTimeout(ctx, d.probeBudget)
		_ = d.Probe(probeCtx)
		cancel()

		// Inter-probe wait: respect the fail-pause backoff when
		// we hit the failure threshold.
		d.mu.RLock()
		cf := d.consecutiveFailures
		d.mu.RUnlock()
		wait := d.probeInterval
		if cf >= d.failThreshold {
			wait = d.failPause
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
	}
}

// Recheck is the HTTP-driven trigger. Returns ErrProbeInFlight if
// a probe is already running. The handler serves the current
// snapshot in that case.
func (d *DynamicLoader) Recheck(ctx context.Context) error {
	return d.Probe(ctx)
}

// Close stops the background loop and signals the in-flight
// probe subprocess (if any) to exit. Idempotent.
func (d *DynamicLoader) Close() {
	d.muStart.Lock()
	defer d.muStart.Unlock()
	if !d.started {
		return
	}
	d.started = false
	if d.cancel != nil {
		d.cancel()
	}
	d.cmdMu.Lock()
	if d.cmd != nil && d.cmd.Process != nil {
		_ = d.cmd.Process.Signal(syscall.SIGTERM)
	}
	d.cmdMu.Unlock()
}
