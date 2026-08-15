package omp

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
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
}

// Session is a live stdio session with an omp subprocess.
type Session struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser

	mu              sync.Mutex
	closed          bool
	protocolVersion int
	ompVersion      string
	ompBin          string
}

// Reader returns the structured stdout scanner.
func (s *Session) Reader() io.Reader { return s.stdout }

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

	hsCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	handshake, err := readHandshake(hsCtx, bufio.NewReader(stdout))
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

	return &Session{
		cmd:             cmd,
		stdin:           stdin,
		stdout:          stdout,
		stderr:          stderr,
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
