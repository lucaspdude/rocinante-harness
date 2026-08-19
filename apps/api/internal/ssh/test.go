package ssh

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// TestOutcome is the categorical result of a TestConnection call.
// The web renders one of the matching i18n strings for each
// outcome, so the labels here are the source of truth.
type TestOutcome string

const (
	OutcomeOK           TestOutcome = "ok"
	OutcomeAuthFailed   TestOutcome = "auth_failed"
	OutcomeConnRefused  TestOutcome = "conn_refused"
	OutcomeNetwork      TestOutcome = "network"
	OutcomeNotInstalled TestOutcome = "not_installed"
)

// TestResult is the value returned by TestConnection. Outcome is
// the categorical state; Message is the raw stderr so the UI can
// surface the underlying ssh error verbatim when it is useful.
type TestResult struct {
	Outcome TestOutcome
	Message string
}

// IdentityForKey returns the on-disk path to the identity file for
// the given key, identified by its db id. The handler uses this to
// pass the resolved path into TestConnection so the test helper
// stays focused on running ssh and classifying the outcome.
//
// The lookup walks the KeyStore; there is no separate index, so the
// cost is O(N) over the keys list. With ~10 keys per install this
// is fine — fall back to a memoised map if the panel grows.
func IdentityForKey(ks *KeyStore, sshDir, id string) (string, error) {
	if ks == nil {
		return "", errors.New("keys store is nil")
	}
	keys, err := ks.List()
	if err != nil {
		return "", err
	}
	for _, k := range keys {
		if k.ID == id {
			return identityPath(sshDir, k.Label), nil
		}
	}
	return "", errors.New("key_not_found")
}

// TestConnection runs a single ssh probe against the given server
// and classifies the outcome. It uses the -o BatchMode=yes flag so
// the command never blocks on a password/passphrase prompt and
// exits as soon as the result is known.
//
// The probe is intentionally trivial: `ssh -T -o BatchMode=yes -o
// ConnectTimeout=5 user@host -p port true` (with -i <identity_file>
// when the caller supplied one). We never actually allocate a PTY
// because -T disables it; this means the command runs faster and
// we get a clean exit code from the remote shell.
//
// Classification rules (in order):
//
//	255 + "Permission denied (publickey)"          → AUTH_FAILED
//	255 + "Connection refused"                     → CONN_REFUSED
//	255 + "Could not resolve hostname"
//	     | "Connection timed out"
//	     | "No route to host"                      → NETWORK
//	0                                               → OK
//	exec error (binary missing)                    → NOT_INSTALLED
//	anything else                                  → NETWORK (defensive fallback)
//
// identityPath is the resolved path to the ssh key file. When
// empty we pass no -i flag and let ssh use the user default
// (~/.ssh/id_* or the agent). That path is what the spec calls
// out as "no key selected" — the api still works because pubkey
// auth is the only path we support.
func TestConnection(ctx context.Context, s Server, identityPath string) TestResult {
	// Build the argv. We always set -o BatchMode=yes and an explicit
	// ConnectTimeout so the call never hangs past 5 seconds. The
	// host fingerprint is not pinned — StrictHostKeyChecking=accept-new
	// lets first-time connects succeed without an interactive prompt.
	args := []string{
		"-T",
		"-o", "BatchMode=yes",
		"-o", "ConnectTimeout=5",
		"-o", "StrictHostKeyChecking=accept-new",
		"-p", strconv.Itoa(s.Port),
	}
	if identityPath != "" {
		if _, err := os.Stat(identityPath); err == nil {
			args = append(args, "-i", identityPath)
		}
	}
	args = append(args, s.Username+"@"+s.Host)
	args = append(args, "true")

	// Defensive: if the caller passes a nil context, fall back to
	// the request context so the handler still bounds the call.
	if ctx == nil {
		ctx = context.Background()
	}
	// Hard 7s deadline on top of the ssh internal timeout so a
	// hung process can't pin the goroutine.
	ctx, cancel := context.WithTimeout(ctx, 7*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "ssh", args...)
	var stderr, stdout bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Stdout = &stdout
	err := cmd.Run()

	if err == nil {
		return TestResult{Outcome: OutcomeOK}
	}

	// exec.CommandContext returns an *exec.ExitError when the
	// process exits non-zero. We use that to read the exit code
	// and map back to the categorical outcome.
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		// The message we surface is the more informative of
		// stderr/stdout — ssh writes human-readable errors to
		// stderr; the remote command writes to stdout.
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = strings.TrimSpace(stdout.String())
		}
		code := exitErr.ExitCode()
		// 255 is the conventional ssh client error code for
		// transport/auth failures. Signal exits (128+sig) are
		// also routed through classifySSHError.
		if code == 255 || isSignalExit(code) {
			return TestResult{Outcome: classifySSHError(msg), Message: msg}
		}
		// Any other non-zero exit is treated as a network-level
		// failure: the remote shell rejected the command or the
		// connection was reset mid-stream.
		return TestResult{Outcome: OutcomeNetwork, Message: msg}
	}

	// Otherwise the binary itself couldn't run. The most common
	// reason is that ssh is not on PATH. We map this to a
	// distinct outcome so the UI can show "OpenSSH client not
	// installed" instead of a generic network error.
	if isMissingBinary(err) {
		return TestResult{Outcome: OutcomeNotInstalled, Message: err.Error()}
	}
	// Context deadline — the user waited the full 7s with no
	// response. Most likely the host is unreachable or firewalled.
	if errors.Is(err, context.DeadlineExceeded) {
		return TestResult{Outcome: OutcomeNetwork, Message: "timeout"}
	}
	return TestResult{Outcome: OutcomeNetwork, Message: err.Error()}
}

// classifySSHError maps stderr text from a failed ssh invocation
// into a TestOutcome. The strings are what OpenSSH 9.x writes on
// Linux/macOS; we keep the matchers tolerant of partial phrasings.
func classifySSHError(msg string) TestOutcome {
	switch {
	case strings.Contains(msg, "Permission denied (publickey)"),
		strings.Contains(msg, "Permission denied (password)"):
		return OutcomeAuthFailed
	case strings.Contains(msg, "Connection refused"):
		return OutcomeConnRefused
	case strings.Contains(msg, "Could not resolve hostname"),
		strings.Contains(msg, "Name or service not known"),
		strings.Contains(msg, "Connection timed out"),
		strings.Contains(msg, "No route to host"),
		strings.Contains(msg, "Operation timed out"):
		return OutcomeNetwork
	}
	// 255 with an unknown message is treated as a generic network
	// failure rather than auth — the user can re-test with verbose
	// ssh to figure out the real cause.
	return OutcomeNetwork
}

// isMissingBinary reports whether the error from
// exec.CommandContext means the `ssh` binary doesn't exist on PATH.
// exec.LookPath returns an *exec.Error with Err set to fs.ErrNotExist
// (or syscall.ENOENT) when the binary is missing.
func isMissingBinary(err error) bool {
	var exitErr *exec.Error
	if errors.As(err, &exitErr) {
		if errors.Is(exitErr.Err, os.ErrNotExist) ||
			errors.Is(exitErr.Err, syscall.ENOENT) {
			return true
		}
	}
	// Some Go versions wrap the path error; fall back to substring
	// match in case the error type changed.
	return strings.Contains(err.Error(), "executable file not found") ||
		strings.Contains(err.Error(), "no such file")
}

// isSignalExit returns true for the conventional "ssh died from a
// signal" exit codes (128 + signal number). We use this to fall
// back to the network classification when the process got killed
// rather than completing normally.
func isSignalExit(code int) bool {
	return code >= 128 && code < 160
}
