package clitools

// Spawn + stdio pipes + line scan + URL/code regex capture.
//
// Runner is the seam between the api handler (which receives
// HTTP requests) and the operating system process. It keeps
// no state of its own; all per-job state lives on the *Job
// value passed in. Concurrent invocations on distinct jobs
// are independent.

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// CmdFactory is the seam used to construct *exec.Cmd. Tests
// inject a mock so they don't have to actually fork a binary.
// Mirrors the api.CmdIface pattern from PR-01 (login flow).
type CmdFactory func(ctx context.Context, name string, args ...string) cmdIface

// defaultCmdFactory wraps exec.CommandContext.
func defaultCmdFactory(ctx context.Context, name string, args ...string) cmdIface {
	return &osCmdWrapper{Cmd: exec.CommandContext(ctx, name, args...)}
}

// osCmdWrapper adapts *exec.Cmd to cmdIface. Defined as a
// struct (not direct *exec.Cmd) so tests can substitute
// without depending on the production type.
type osCmdWrapper struct {
	Cmd *exec.Cmd
}

func (c *osCmdWrapper) StdinPipe() (io.WriteCloser, error)  { return c.Cmd.StdinPipe() }
func (c *osCmdWrapper) StdoutPipe() (io.ReadCloser, error) { return c.Cmd.StdoutPipe() }
func (c *osCmdWrapper) StderrPipe() (io.ReadCloser, error)  { return c.Cmd.StderrPipe() }
func (c *osCmdWrapper) Start() error                        { return c.Cmd.Start() }
func (c *osCmdWrapper) Wait() error                         { return c.Cmd.Wait() }
func (c *osCmdWrapper) Process() *os.Process                { return c.Cmd.Process }

// RunChild is the all-in-one spawn-and-watch call. It forks
// the process, wires stdout+stderr to line scanners, runs the
// monitor goroutine, and returns the *Job (with Pid set) to
// the handler. The monitor closes the job when the child
// exits.
//
// authURL/authCode are populated by the monitor's regex pass
// for Login jobs; the HTTP handler reads them on the next
// poll. For Install jobs, the regex pass is skipped.
//
// The runner takes a stdioSink callback (may be nil) that
// receives each captured line before it's appended to the
// ring buffer. Tests use this to assert on captured output.
func RunChild(ctx context.Context, jobs *Jobs, factory CmdFactory, cliID string, kind JobKind, argv []string, onLine func(line string)) (*Job, error) {
	if len(argv) == 0 {
		return nil, fmt.Errorf("clitools: empty argv for %s/%s", cliID, kind)
	}
	job := jobs.NewJob(cliID, kind)
	cmdCtx, cancel := context.WithCancel(ctx)
	cmd := factory(cmdCtx, argv[0], argv[1:]...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("clitools: stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("clitools: stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("clitools: stderr pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("clitools: start: %w", err)
	}
	proc := cmd.Process()
	if proc != nil {
		job.Pid = proc.Pid
	}
	job.cancel = cancel
	// Stash the running cmd for both the production exec.Cmd
	// and the test cmdIface. waitForChild uses the iface to
	// call Wait(); WriteAck uses job.child.Stdin for the
	// pipe.
	if w, ok := cmd.(*osCmdWrapper); ok {
		job.child = w.Cmd
	}
	job.iface = cmd
	job.stdinMu.Lock()
	job.stdin = stdin
	job.stdinMu.Unlock()

	spec, _ := GetSpec(cliID)
	go monitor(job, spec, stdout, stderr, onLine, cancel)
	return job, nil
}

// stdinWriter is the lock-guarded accessor for the job's
// stdin pipe. Returns nil when the pipe has been closed
// (e.g. the runner tore it down after the child exited).
func (j *Job) stdinWriter() io.Writer {
	j.stdinMu.Lock()
	defer j.stdinMu.Unlock()
	if j.stdin == nil {
		return nil
	}
	return j.stdin
}

// stdinClose closes the job's stdin writer; idempotent.
func (j *Job) stdinClose() {
	j.stdinMu.Lock()
	if j.stdin != nil {
		_ = j.stdin.Close()
		j.stdin = nil
	}
	j.stdinMu.Unlock()
}

// monitor drives a single spawned job to completion. It
// reads stdout + stderr line-by-line, appends each line to
// the ring buffer, applies the spec's regexes for Login jobs,
// and finalizes the job when the child exits.
//
// The stdioSink callback fires before the line is appended
// so tests can assert on output ordering. nil disables the
// hook (production path).
func monitor(job *Job, spec Spec, stdout, stderr io.Reader, sink func(string), cancel context.CancelFunc) {
	// spec.LoginTimeoutSeconds governs both install and login
	// wall clock in v2 — keeps the runner simple. 0 = no
	// timeout.
	var deadline <-chan time.Time
	if spec.LoginTimeoutSeconds > 0 {
		t := time.NewTimer(time.Duration(spec.LoginTimeoutSeconds) * time.Second)
		defer t.Stop()
		deadline = t.C
	}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		scanPipe(stdout, job, spec, sink)
	}()
	go func() {
		defer wg.Done()
		scanPipe(stderr, job, spec, sink)
	}()

	waitErr := waitForChild(job, deadline, cancel)
	// After the child exits, close the reader side of the
	// pipes so scanPipe's scanner returns EOF and the
	// goroutines exit. Production io.Pipe readers don't
	// need this; tests do (the mock's read ends stick
	// around if the goroutine is leaked past the test
	// function).
	if closer, ok := stdout.(io.Closer); ok {
		_ = closer.Close()
	}
	if closer, ok := stderr.(io.Closer); ok {
		_ = closer.Close()
	}
	wg.Wait()



	if waitErr != nil {
		// Non-zero exit OR timeout-driven cancel.
		if job.Status == JobRunning {
			ec := -1
			job.ExitCode = &ec
			job.MarkFailed(waitErr.Error())
		}
		return
	}
	job.MarkDone(0)
}

// waitForChild blocks until the child exits or the deadline
// fires. Returns nil on a clean exit, an error otherwise.
// The cancel func is invoked on timeout.
func waitForChild(job *Job, deadline <-chan time.Time, cancel context.CancelFunc) error {
	done := job.Done()
	if job.iface == nil {
		// No real child to wait on; just block until the
		// job is marked done externally.
		<-done
		return nil
	}
	errCh := make(chan error, 1)
	go func() { errCh <- job.iface.Wait() }()
	select {
	case err := <-errCh:
		return err
	case <-deadline:
		if cancel != nil {
			cancel()
		}
		// Drain the wait so we don't leak the goroutine.
		<-errCh
		return fmt.Errorf("timeout after %v", deadline)
	}
}

// scanPipe reads a pipe line-by-line. Each captured line is
// passed to the optional sink (tests) and appended to the
// job's ring buffer. For Login jobs, the spec's URL + code
// regexes are applied to extract the device-code prompt.
func scanPipe(r io.Reader, job *Job, spec Spec, sink func(string)) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if sink != nil {
			sink(line)
		}
		job.AppendLine(line)
		if job.Kind == Login {
			if spec.LoginURLRegex != nil {
				if m := spec.LoginURLRegex.FindStringSubmatch(line); len(m) >= 2 {
					job.SetAuth(m[1], job.AuthCode)
				}
			}
			if spec.LoginCodeRegex != nil {
				if m := spec.LoginCodeRegex.FindStringSubmatch(line); len(m) >= 2 {
					job.SetAuth(job.AuthURL, strings.TrimSpace(m[1]))
				}
			}
		}
	}
}

// WriteAck writes value to the job's child stdin. Used by
// the ack handler when the user pastes their device-code
// confirmation. The child must still be running; otherwise
// the write fails silently with EPIPE.
func WriteAck(job *Job, value string) error {
	if job == nil {
		return fmt.Errorf("clitools: nil job")
	}
	w := job.stdinWriter()
	if w == nil {
		return fmt.Errorf("clitools: stdin already closed")
	}
	_, err := io.WriteString(w, value)
	return err
}

// WriteAutoAck writes spec.LoginAck (typically "\n" or "")
// to the job's stdin. Called by the login-start handler right
// after spawning the child.
func WriteAutoAck(job *Job, spec Spec) error {
	if spec.LoginAck == "" {
		return nil
	}
	return WriteAck(job, spec.LoginAck)
}

// Cancel kills the job's child if still running.
func Cancel(job *Job) {
	if job == nil || job.cancel == nil {
		return
	}
	job.cancel()
}