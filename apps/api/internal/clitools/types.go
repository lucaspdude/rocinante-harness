// Package clitools installs and signs in to third-party CLIs
// (az, gh) from the Settings → Developer tools → CLIs panel.
//
// Each provider is described by a Spec that captures install
// recipes, status-probe argv, and the device-code OAuth flow
// (regexes for URL + code, ack prompt). Jobs are short-lived;
// the registry is an in-memory map keyed by job id and is GC'd
// 30 minutes after completion.
//
// Design notes (PR-06):
//   - In-memory only: jobs die on api restart (matches the
//     reference's lib/cli-tools/jobs.ts). Persistence adds
//     complexity for no real benefit; CLI install/login take ≤2
//     minutes and the api restarts rarely.
//   - macOS only for v2: linux install recipes use `sudo apt`,
//     which the harness api process on a fresh LXC may not have
//     permission for. Defer per OQ5.
//   - The runner exposes *exec.Cmd via job.child so the ack
//     handler can pipe the user-supplied code back into the
//     child (az expects the user to type the code, gh expects
//     just a newline after the "Press Enter" prompt).
package clitools

import (
	"context"
	"io"
	"os"
	"os/exec"
	"regexp"
	"sync"
)

// JobKind is whether the job installs or signs in.
type JobKind string

const (
	Install JobKind = "install"
	Login   JobKind = "login"
)

// JobState is the lifecycle state of a single install/login job.
type JobState string

const (
	JobRunning JobState = "running"
	JobDone    JobState = "done"
	JobFailed  JobState = "failed"
)

// cmdIface is the subset of *exec.Cmd the runner needs. Kept
// narrow so the tests can stub a no-op implementation. Lives
// inside the package so the runner can refer to it without
// depending on the api package.
type cmdIface interface {
	StdinPipe() (io.WriteCloser, error)
	StdoutPipe() (io.ReadCloser, error)
	StderrPipe() (io.ReadCloser, error)
	Start() error
	Wait() error
	Process() *os.Process
}

// Job holds the in-flight state of a spawn-and-watch process.
// Lines is a ring buffer capped at ringCap; AuthURL / AuthCode
// are populated by the monitor goroutine when the spec's
// regexes match the child's output.
type Job struct {
	ID        string
	CliID     string
	Kind      JobKind
	Pid       int
	StartedAt int64 // unix seconds
	Status    JobState
	Lines     []string
	ExitCode  *int
	AuthURL   string
	AuthCode  string
	Stream    string // "stderr" or "stdout" — which pipe the device-code prompt came from

	// Internal: protected by muLines / muSub / stdinMu.
	muLines sync.Mutex
	muSub   sync.Mutex
	stdinMu sync.Mutex
	cancel  context.CancelFunc
	child   *exec.Cmd
	iface   cmdIface
	stdin   io.WriteCloser
	subs    map[int]chan struct{}
	subSeq  int
	done    chan struct{}
}

// Spec describes one provider. Install / LoginCmd are argv
// slices (no shell). The login flow reads stdout/stderr line
// by line and applies LoginURLRegex + LoginCodeRegex to
// capture the device-code URL + code.
type Spec struct {
	ID                  string
	DisplayName         string
	HelpText            string
	Install             map[string][]string // "mac" / "linux" / "win"
	VerifyInstall       []string
	VerifyAuth          []string
	AccountQuery        []string
	LoginCmd            []string
	LoginStream         string         // "stderr" or "stdout"
	LoginURLRegex       *regexp.Regexp // first submatch is the URL
	LoginCodeRegex      *regexp.Regexp // first submatch is the code
	LoginAck            string         // "" or "\n" — auto-ack on job start
	LoginTimeoutSeconds int            // 0 = no timeout; default 900
}

// Platform returns the install argv for the given runtime.GOOS,
// or nil if the platform has no install recipe (e.g. linux/win
// for v2).
func (s Spec) Platform(goos string) []string {
	return s.Install[goos]
}

// ringCap is the max number of Lines the runner keeps in the
// job's ring buffer. New lines push older ones out.
const ringCap = 200