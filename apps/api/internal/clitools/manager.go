package clitools

// Manager is the orchestrator for the cli-tools feature. It
// owns the in-memory job store, the spawn factory, and the
// platform-detection shim. HTTP handlers reach the public
// API through a *Manager; tests can build their own with
// Manager{ Jobs: ..., Factory: ... }.

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// Manager is the entry point for HTTP handlers. Safe for
// concurrent use; the underlying job store has its own lock.
type Manager struct {
	Jobs    *Jobs
	Factory CmdFactory
	// Platform is the runtime.GOOS-equivalent string the
	// specs use as a map key. Defaults to runtime.GOOS on
	// construction; tests can override it to simulate
	// different platforms.
	Platform string
}

// NewManager returns a Manager backed by the production
// command factory and the current GOOS.
func NewManager() *Manager {
	return &Manager{
		Jobs:     NewJobs(),
		Factory:  defaultCmdFactory,
		Platform: runtime.GOOS,
	}
}

// InstallStart spawns the platform install command for cliID
// and returns the resulting job. The caller (HTTP handler)
// returns 202 with job_id / pid; the SSE handler subscribes
// to the same job and streams log lines.
//
// Returns ErrUnsupportedPlatform if the spec has no install
// recipe for the current GOOS.
func (m *Manager) InstallStart(ctx context.Context, cliID string) (*Job, error) {
	spec, ok := GetSpec(cliID)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnknownCli, cliID)
	}
	argv := spec.Platform(m.Platform)
	if len(argv) == 0 {
		return nil, fmt.Errorf("%w: %s/%s", ErrUnsupportedPlatform, cliID, m.Platform)
	}
	return RunChild(ctx, m.Jobs, m.Factory, cliID, Install, argv, nil)
}

// LoginStart spawns the spec's LoginCmd for cliID and returns
// the resulting job. The monitor's regex pass extracts the
// device-code URL + code; the HTTP handler reads them off
// the job once the runner has captured them (typically <1s).
func (m *Manager) LoginStart(ctx context.Context, cliID string) (*Job, error) {
	spec, ok := GetSpec(cliID)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnknownCli, cliID)
	}
	if len(spec.LoginCmd) == 0 {
		return nil, fmt.Errorf("%w: %s has no LoginCmd", ErrUnsupportedPlatform, cliID)
	}
	job, err := RunChild(ctx, m.Jobs, m.Factory, cliID, Login, spec.LoginCmd, nil)
	if err != nil {
		return nil, err
	}
	// gh's "Press Enter to open the browser" prompt blocks
	// on stdin BEFORE printing the URL+code. Spec encodes
	// the auto-ack as "\n"; write it here.
	if err := WriteAutoAck(job, spec); err != nil {
		// Non-fatal — the child may have already read EOF
		// or moved on. The job still proceeds; auth will
		// fail at the device-code prompt instead.
		_ = err
	}
	return job, nil
}

// LoginAck writes the user-supplied device-code confirmation
// into the running child's stdin. The cli prints the URL +
// code, the user opens the URL and pastes the code back; the
// api pipes it into the child so the login can complete.
func (m *Manager) LoginAck(jobID, value string) error {
	job := m.Jobs.Get(jobID)
	if job == nil {
		return ErrJobNotFound
	}
	if job.Kind != Login {
		return fmt.Errorf("clitools: ack requires Login job, got %s", job.Kind)
	}
	return WriteAck(job, value)
}

// Status probes the local host for install/auth state by
// running the spec's VerifyInstall + VerifyAuth + AccountQuery
// commands. StatusResponse mirrors the JSON shape the web
// panel expects.
func (m *Manager) Status(ctx context.Context, cliID string) StatusResponse {
	resp := StatusResponse{CliID: cliID}
	spec, ok := GetSpec(cliID)
	if !ok {
		resp.Detail = "unknown_cli"
		return resp
	}
	// verify-install
	out, err := m.runProbe(ctx, spec.VerifyInstall)
	if err != nil {
		resp.Installed = false
		if ee, ok := err.(*exec.ExitError); ok {
			resp.Detail = fmt.Sprintf("install probe exit %d", ee.ExitCode())
		} else if isNotFound(err) {
			resp.Detail = cliID + ": command not found"
		} else {
			resp.Detail = err.Error()
		}
		return resp
	}
	resp.Installed = true
	// version is the first line of the install probe output,
	// trimmed of trailing whitespace. Matches the reference's
	// behaviour ("azure-cli 2.65.0" etc.).
	if len(out) > 0 {
		resp.Version = strings.TrimSpace(out[0])
	}
	// verify-auth
	if _, err := m.runProbe(ctx, spec.VerifyAuth); err == nil {
		resp.Authenticated = true
	} else if isNotFound(err) {
		resp.Detail = cliID + ": command not found"
	} else {
		resp.Authenticated = false
	}
	// account-query
	if resp.Authenticated && len(spec.AccountQuery) > 0 {
		if out, err := m.runProbe(ctx, spec.AccountQuery); err == nil && len(out) > 0 {
			resp.Account = strings.TrimSpace(out[0])
		}
	}
	return resp
}

// runProbe runs argv and returns the trimmed output lines.
// Exits are tolerated: a non-zero exit produces an *exec.ExitError
// which the caller maps to authenticated=false or
// installed=false. A "not found" error (lookPath returns
// ErrNotFound) surfaces as isNotFound=true.
func (m *Manager) runProbe(ctx context.Context, argv []string) ([]string, error) {
	if len(argv) == 0 {
		return nil, fmt.Errorf("clitools: empty probe argv")
	}
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	lines := splitLines(string(out))
	return lines, nil
}

// splitLines splits on \n and trims trailing \r. The probe
// argv typically print a single line; we keep it general so
// future specs (multi-line versions, etc.) work without a
// shape change.
func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(strings.TrimRight(s, "\n"), "\n")
	for i, p := range parts {
		parts[i] = strings.TrimRight(p, "\r")
	}
	return parts
}

// isNotFound returns true when err is the *exec.Error whose
// Name field indicates the binary was not on PATH. Used to
// give the user a cleaner "command not found" message
// instead of an opaque Go error.
func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	if ee, ok := err.(*exec.Error); ok && ee.Err == exec.ErrNotFound {
		return true
	}
	if err == exec.ErrNotFound {
		return true
	}
	// Fallback: the wrapped string sometimes shows up when
	// the cmd is wrapped via CmdFactory in tests.
	return strings.Contains(err.Error(), "executable file not found")
}

// StatusResponse is the JSON shape the web panel renders.
type StatusResponse struct {
	CliID         string `json:"id"`
	Installed     bool   `json:"installed"`
	Authenticated bool   `json:"authenticated"`
	Version       string `json:"version,omitempty"`
	Account       string `json:"account,omitempty"`
	Detail        string `json:"detail,omitempty"`
}

// Errors surfaced by the manager. Handlers map these to
// HTTP status codes via errors.Is.
var (
	ErrUnknownCli         = fmt.Errorf("clitools: unknown cli")
	ErrUnsupportedPlatform = fmt.Errorf("clitools: unsupported platform")
	ErrJobNotFound         = fmt.Errorf("clitools: job_not_found")
)