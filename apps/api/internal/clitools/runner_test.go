package clitools

// Tests for the clitools runner. Use a mock cmdIface so we
// don't fork a real binary.

import (
	"context"
	"io"
	"os"
	"sync"
	"testing"
	"time"
)

// mockCmd implements cmdIface. The constructor wires
// in-memory pipes:
//
//	stdinR  ── runner reads here (after it writes)
//	stdinW  ── runner writes here (we read from stdinR)
//
//	stdoutW ── test writes here (runner reads from stdoutR)
//	stdoutR ── runner reads here
//
//	stderrW ── test writes here
//	stderrR ── runner reads here
//
// The mock's Wait() blocks until the test calls Done() so
// the runner doesn't close stdoutR/stderrR before the test
// has finished writing.
type mockCmd struct {
	stdinR  *io.PipeReader
	stdinW  *io.PipeWriter
	stdoutR *io.PipeReader
	stdoutW *io.PipeWriter
	stderrR *io.PipeReader
	stderrW *io.PipeWriter

	pid   int
	wait  chan struct{} // closed by Done() to release Wait()
	once  sync.Once
	mu    sync.Mutex
	done  bool
}

func newMockCmd() *mockCmd {
	stdoutR, stdoutW := io.Pipe()
	stderrR, stderrW := io.Pipe()
	stdinR, stdinW := io.Pipe()
	return &mockCmd{
		stdinR:  stdinR,
		stdinW:  stdinW,
		stdoutR: stdoutR,
		stdoutW: stdoutW,
		stderrR: stderrR,
		stderrW: stderrW,
		pid:     4242,
		wait:    make(chan struct{}),
	}
}

// StdinPipe: returns the write end the runner uses.
func (m *mockCmd) StdinPipe() (io.WriteCloser, error) { return m.stdinW, nil }

// StdoutPipe: returns the read end the runner reads from.
func (m *mockCmd) StdoutPipe() (io.ReadCloser, error) { return m.stdoutR, nil }

// StderrPipe: returns the read end the runner reads from.
func (m *mockCmd) StderrPipe() (io.ReadCloser, error) { return m.stderrR, nil }

// Start: just record. The mock is "running" until Done() is called.
func (m *mockCmd) Start() error { return nil }

// Wait: blocks until the test calls Done().
func (m *mockCmd) Wait() error {
	<-m.wait
	m.mu.Lock()
	m.done = true
	m.mu.Unlock()
	return nil
}

// Done: releases Wait(). Safe to call once.
func (m *mockCmd) Done() {
	m.once.Do(func() { close(m.wait) })
}

// Process: returns nil so the runner treats the mock as
// "no real PID available" — only the test cares.
func (m *mockCmd) Process() *os.Process { return nil }

// asIface wraps a *mockCmd in the cmdIface interface.
func asIface(m *mockCmd) cmdIface { return m }

// factoryFromMock returns a CmdFactory that always yields m.
func factoryFromMock(m *mockCmd) CmdFactory {
	return func(ctx context.Context, name string, args ...string) cmdIface {
		return asIface(m)
	}
}

// finishMock: writes remaining data, closes the pipes, then
// calls mock.Done() so Wait() returns. Returns once the job
// is finished (or fails the test on timeout).
func finishMock(t *testing.T, mc *mockCmd, job *Job, stdoutText, stderrText string) JobSnapshot {
	t.Helper()
	// Write any pending data.
	if stdoutText != "" {
		_, _ = io.WriteString(mc.stdoutW, stdoutText)
	}
	if stderrText != "" {
		_, _ = io.WriteString(mc.stderrW, stderrText)
	}
	// Close write ends so the runner's readers return EOF.
	// We DON'T close stdinR; the test uses it directly.
	mc.stdoutW.Close()
	mc.stderrW.Close()
	// Release Wait() so the runner sees the child exit.
	mc.Done()
	return waitForFinish(t, job, 3*time.Second)
}

func waitForFinish(t *testing.T, job *Job, timeout time.Duration) JobSnapshot {
	t.Helper()
	deadline := time.After(timeout)
	for {
		select {
		case <-deadline:
			t.Fatalf("job did not finish in time: %#v", job.Snapshot())
		default:
		}
		s := job.Snapshot()
		if s.Status != JobRunning {
			return s
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestRunChildCapturesStdoutAndStderr(t *testing.T) {
	mc := newMockCmd()
	jobs := NewJobs()
	job, err := RunChild(context.Background(), jobs, factoryFromMock(mc), "fake", Install, []string{"fake", "--x"}, nil)
	if err != nil {
		t.Fatalf("RunChild: %v", err)
	}
	snap := finishMock(t, mc, job, "hello-stdout\n", "hello-stderr\n")
	if snap.Status != JobDone {
		t.Errorf("status = %s, want done", snap.Status)
	}
	combined := ""
	for _, l := range snap.Lines {
		combined += l + "\n"
	}
	if !contains(combined, "hello-stdout") {
		t.Errorf("missing stdout: %s", combined)
	}
	if !contains(combined, "hello-stderr") {
		t.Errorf("missing stderr: %s", combined)
	}
}

func TestRunChildGhAuthURLAndCode(t *testing.T) {
	// gh auth login --no-browser prints the one-time code
	// first, then (after Enter) the device URL. We capture
	// both lines so the runner's regex pass populates
	// AuthURL and AuthCode.
	const ghPrompt = "! First copy your one-time code: ABCD-1234\nOpen this URL: https://github.com/login/device\n"
	mc := newMockCmd()
	jobs := NewJobs()
	spec := CLIS["gh"]
	job, err := RunChild(context.Background(), jobs, factoryFromMock(mc), "gh", Login, spec.LoginCmd, nil)
	if err != nil {
		t.Fatalf("RunChild: %v", err)
	}
	// Write the prompt first so scanPipe populates AuthURL
	// + AuthCode, THEN release Wait() so the runner marks
	// the job done.
	finishMock(t, mc, job, ghPrompt, "")
	snap := job.Snapshot()
	if snap.AuthCode != "ABCD-1234" {
		t.Errorf("AuthCode = %q, want ABCD-1234", snap.AuthCode)
	}
	if snap.AuthURL != "https://github.com/login/device" {
		t.Errorf("AuthURL = %q", snap.AuthURL)
	}
}

func TestRunChildAzAuthURLAndCode(t *testing.T) {
	const azPrompt = "To sign in, use a web browser to open the page https://microsoft.com/devicelogin and enter the code ABC123 to authenticate.\n"
	mc := newMockCmd()
	jobs := NewJobs()
	spec := CLIS["az"]
	job, err := RunChild(context.Background(), jobs, factoryFromMock(mc), "az", Login, spec.LoginCmd, nil)
	if err != nil {
		t.Fatalf("RunChild: %v", err)
	}
	finishMock(t, mc, job, "", azPrompt)
	snap := job.Snapshot()
	if snap.AuthCode != "ABC123" {
		t.Errorf("AuthCode = %q, want ABC123", snap.AuthCode)
	}
	if snap.AuthURL != "https://microsoft.com/devicelogin" {
		t.Errorf("AuthURL = %q", snap.AuthURL)
	}
}

func TestWriteAutoAckGh(t *testing.T) {
	// gh needs a newline after "Press Enter"; exercise the
	// auto-ack path on a live mock.
	mc := newMockCmd()
	jobs := NewJobs()
	spec := CLIS["gh"]
	job, err := RunChild(context.Background(), jobs, factoryFromMock(mc), "gh", Login, spec.LoginCmd, nil)
	if err != nil {
		t.Fatalf("RunChild: %v", err)
	}
	// io.Pipe.Write blocks until a Read drains. Read in a
	// goroutine so WriteAutoAck can return.
	type readResult struct {
		val string
		err error
	}
	stdinRead := make(chan readResult, 1)
	go func() {
		buf := make([]byte, 8)
		n, err := mc.stdinR.Read(buf)
		stdinRead <- readResult{val: string(buf[:n]), err: err}
	}()
	if err := WriteAutoAck(job, spec); err != nil {
		t.Fatalf("WriteAutoAck: %v", err)
	}
	select {
	case res := <-stdinRead:
		if res.err != nil && res.err != io.EOF {
			t.Fatalf("stdin read: %v", res.err)
		}
		if res.val != "\n" {
			t.Errorf("stdin = %q, want \\n", res.val)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("stdin read timed out")
	}
	// Drain pipes so the monitor can finish.
	finishMock(t, mc, job, "", "")
}

func TestJobsSweepEvictsOldJobs(t *testing.T) {
	jobs := NewJobs()
	j := jobs.NewJob("az", Install)
	j.Status = JobDone
	// StartedAt was just now; sweep should keep it.
	jobs.Sweep()
	if jobs.Get(j.ID) == nil {
		t.Fatalf("fresh job evicted")
	}
	// Backdate and re-sweep.
	j.StartedAt = time.Now().Add(-2 * time.Hour).Unix()
	jobs.Sweep()
	if jobs.Get(j.ID) != nil {
		t.Fatalf("old job not evicted")
	}
}

func TestSpecsLookup(t *testing.T) {
	for _, id := range []string{"az", "gh"} {
		spec, ok := GetSpec(id)
		if !ok {
			t.Errorf("missing spec for %s", id)
			continue
		}
		if spec.LoginCmd == nil {
			t.Errorf("spec %s has nil LoginCmd", id)
		}
		if spec.VerifyInstall == nil {
			t.Errorf("spec %s has nil VerifyInstall", id)
		}
	}
}

func TestStatusProbeCommandNotFound(t *testing.T) {
	mgr := &Manager{Jobs: NewJobs(), Factory: defaultCmdFactory, Platform: "darwin"}
	resp := mgr.Status(context.Background(), "definitely-not-installed-cli")
	if resp.Installed {
		t.Errorf("expected installed=false")
	}
	if resp.Detail == "" {
		t.Errorf("expected non-empty detail")
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}