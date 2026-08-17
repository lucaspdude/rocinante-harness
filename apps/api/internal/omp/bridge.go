package omp

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

// Options configures a new omp Session.
type Options struct {
	OpBin  string
	OmpCwd string
	Cwd    string
	Model  string
	// Env, if non-empty, is appended to the api's own
	// os.Environ() and passed to the omp subprocess. The api
	// uses this to inject provider keys from the keystore on
	// every spawn — the subprocess sees the keys without the
	// api process having to be restarted. The values are
	// expected to be in "KEY=VALUE" form, one per entry.
	Env []string
}

// Session is a live stdio session with an omp subprocess.
type Session struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser

	// combined stitches the bytes the bufio.Reader buffered past
	// the first newline (during the handshake) with the live
	// stdout pipe. Session.Reader() returns this so the SSE
	// handler sees every frame omp produced.
	combined io.Reader

	mu              sync.Mutex
	closed          bool
	protocolVersion int
	ompVersion      string
	ompBin          string
}

// Reader returns the structured stdout scanner.
func (s *Session) Reader() io.Reader { return s.combined }

// Writer returns the stdin pipe.
func (s *Session) Writer() io.Writer { return s.stdin }

// Pid returns the OS process id of the omp subprocess.
func (s *Session) Pid() int {
	if s.cmd == nil || s.cmd.Process == nil {
		return 0
	}
	return s.cmd.Process.Pid
}

// Version returns the protocol version and omp version discovered
// during the handshake.
func (s *Session) Version() (int, string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.protocolVersion, s.ompVersion
}

// Close sends SIGTERM to the subprocess, waits up to 3s, then
// escalates to SIGKILL. Idempotent.
func (s *Session) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	cmd := s.cmd
	s.mu.Unlock()
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	_ = cmd.Process.Signal(syscall.SIGTERM)
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-done:
		return nil
	case <-time.After(3 * time.Second):
		_ = cmd.Process.Kill()
		<-done
		return errors.New("omp subprocess did not exit within 3s; SIGKILL sent")
	}
}

// Spawn starts an omp subprocess and runs the handshake. The
// returned Session is ready for read/write but has not yet received
// any application-level commands (those arrive in P2).
func Spawn(ctx context.Context, opts Options) (*Session, error) {
	if opts.OpBin == "" {
		return nil, ErrOmpNotFound
	}
	cmd := exec.CommandContext(ctx, opts.OpBin, "--mode", "rpc-ui")
	if opts.Cwd != "" {
		cmd.Dir = opts.Cwd
	}
	if len(opts.Env) > 0 {
		// Start from the api's own env and append the
		// keystore-supplied entries. Letting the api's env
		// through means omp still sees PATH, HOME, and any
		// other infra the api was started with; the
		// append overrides win for any duplicate keys
		// (keystore is the source of truth for providers).
		cmd.Env = append(os.Environ(), opts.Env...)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("stderr pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("omp start: %w", err)
	}

	br := bufio.NewReader(stdout)
	hsCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	handshake, leftover, err := readHandshakeWithLeftover(hsCtx, br)
	if err != nil {
		_ = cmd.Process.Signal(syscall.SIGTERM)
		_, _ = cmd.Process.Wait()
		return nil, fmt.Errorf("handshake: %w", err)
	}

	ompVersion := handshake.OmpVersion
	if ompVersion == "" && handshake.ProtocolVersion == 1 {
		fallback, ferr := fallbackOmpVersion(ctx, opts.OpBin)
		if ferr == nil {
			ompVersion = fallback
		}
	}

	_ = stderr

	combined := io.MultiReader(bytes.NewReader(leftover), stdout)
	return &Session{
		cmd:             cmd,
		stdin:           stdin,
		stdout:          stdout,
		stderr:          stderr,
		combined:        combined,
		protocolVersion: handshake.ProtocolVersion,
		ompVersion:      ompVersion,
		ompBin:          opts.OpBin,
	}, nil
}

// Loader supplies metadata for the /api/v1/meta handler.
type Loader interface {
	OmpBin() string
	OmpVersion() (int, string)
}
