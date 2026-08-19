package ssh

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// writeFakeSSH writes a small shell script to a temp dir that
// simulates the `ssh` binary. The script chooses its exit code
// based on the FAKE_SSH_MODE env var so each test can dial in the
// behaviour it wants:
//
//	ok             → exit 0
//	auth_failed    → exit 255, stderr "Permission denied (publickey)"
//	conn_refused   → exit 255, stderr "ssh: connect to host x: Connection refused"
//	network        → exit 255, stderr "ssh: connect to host x: Connection timed out"
//	resolve        → exit 255, stderr "ssh: Could not resolve hostname x: Name or service not known"
//	unknown_255    → exit 255, stderr "weird error"
//
// The script is exec'd in place of the real ssh binary by writing
// the temp dir to PATH ahead of the system PATH.
func writeFakeSSH(t *testing.T, mode string) {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "ssh")
	script := `#!/bin/sh
case "$FAKE_SSH_MODE" in
ok)              exit 0 ;;
auth_failed)     echo "Permission denied (publickey)" 1>&2; exit 255 ;;
conn_refused)    echo "ssh: connect to host x port 22: Connection refused" 1>&2; exit 255 ;;
network)         echo "ssh: connect to host x port 22: Connection timed out" 1>&2; exit 255 ;;
resolve)         echo "ssh: Could not resolve hostname x: Name or service not known" 1>&2; exit 255 ;;
unknown_255)     echo "weird error" 1>&2; exit 255 ;;
*)               echo "unknown FAKE_SSH_MODE=$FAKE_SSH_MODE" 1>&2; exit 1 ;;
esac
`
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake ssh: %v", err)
	}
	t.Setenv("FAKE_SSH_MODE", mode)
	// Prepend the fake bin dir to PATH so the subprocess "ssh" picks
	// it up. t.Setenv restores the original PATH after the test.
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestTestConnectionOK(t *testing.T) {
	writeFakeSSH(t, "ok")
	res := TestConnection(context.Background(), Server{
		Host: "127.0.0.1", Port: 22, Username: "u",
	}, "")
	if res.Outcome != OutcomeOK {
		t.Errorf("Outcome = %q, want %q (msg=%q)", res.Outcome, OutcomeOK, res.Message)
	}
}

func TestTestConnectionAuthFailed(t *testing.T) {
	writeFakeSSH(t, "auth_failed")
	res := TestConnection(context.Background(), Server{
		Host: "127.0.0.1", Port: 22, Username: "u",
	}, "")
	if res.Outcome != OutcomeAuthFailed {
		t.Errorf("Outcome = %q, want %q (msg=%q)", res.Outcome, OutcomeAuthFailed, res.Message)
	}
	if res.Message == "" {
		t.Errorf("message empty; should be raw ssh stderr")
	}
}

func TestTestConnectionConnRefused(t *testing.T) {
	writeFakeSSH(t, "conn_refused")
	res := TestConnection(context.Background(), Server{
		Host: "127.0.0.1", Port: 22, Username: "u",
	}, "")
	if res.Outcome != OutcomeConnRefused {
		t.Errorf("Outcome = %q, want %q (msg=%q)", res.Outcome, OutcomeConnRefused, res.Message)
	}
}

func TestTestConnectionNetwork(t *testing.T) {
	writeFakeSSH(t, "network")
	res := TestConnection(context.Background(), Server{
		Host: "127.0.0.1", Port: 22, Username: "u",
	}, "")
	if res.Outcome != OutcomeNetwork {
		t.Errorf("Outcome = %q, want %q (msg=%q)", res.Outcome, OutcomeNetwork, res.Message)
	}
}

func TestTestConnectionResolve(t *testing.T) {
	// "Could not resolve hostname" and "Name or service not known"
	// both map to NETWORK; this covers the cross-platform wording.
	writeFakeSSH(t, "resolve")
	res := TestConnection(context.Background(), Server{
		Host: "does-not-exist.invalid", Port: 22, Username: "u",
	}, "")
	if res.Outcome != OutcomeNetwork {
		t.Errorf("Outcome = %q, want %q (msg=%q)", res.Outcome, OutcomeNetwork, res.Message)
	}
}

func TestTestConnectionUnknownStderr(t *testing.T) {
	// A 255 with an unrecognised stderr should fall back to NETWORK
	// rather than auth. The user can re-test with verbose ssh to
	// figure out the real cause.
	writeFakeSSH(t, "unknown_255")
	res := TestConnection(context.Background(), Server{
		Host: "127.0.0.1", Port: 22, Username: "u",
	}, "")
	if res.Outcome != OutcomeNetwork {
		t.Errorf("Outcome = %q, want %q (msg=%q)", res.Outcome, OutcomeNetwork, res.Message)
	}
}

func TestTestConnectionNotInstalled(t *testing.T) {
	// Point PATH at an empty dir so the real ssh binary can't be
	// found. We use a temp dir that contains no `ssh` script.
	dir := t.TempDir()
	t.Setenv("PATH", dir)
	res := TestConnection(context.Background(), Server{
		Host: "127.0.0.1", Port: 22, Username: "u",
	}, "")
	if res.Outcome != OutcomeNotInstalled {
		t.Errorf("Outcome = %q, want %q (msg=%q)", res.Outcome, OutcomeNotInstalled, res.Message)
	}
}

func TestTestConnectionCancelledContext(t *testing.T) {
	// Cancel the context before invoking TestConnection. The ssh
	// child will be killed by exec.CommandContext; the helper must
	// still return a result rather than hang. We accept either
	// NETWORK (timeout / signal-kill) or NOT_INSTALLED (the
	// subprocess never started) — both are safe outcomes.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	res := TestConnection(ctx, Server{
		Host: "127.0.0.1", Port: 22, Username: "u",
	}, "")
	if res.Outcome != OutcomeNetwork && res.Outcome != OutcomeNotInstalled {
		t.Errorf("Outcome = %q, want %q or %q", res.Outcome, OutcomeNetwork, OutcomeNotInstalled)
	}
}

func TestClassifySSHError(t *testing.T) {
	cases := []struct {
		msg  string
		want TestOutcome
	}{
		{"Permission denied (publickey)", OutcomeAuthFailed},
		{"Permission denied (password)", OutcomeAuthFailed},
		{"ssh: connect to host x port 22: Connection refused", OutcomeConnRefused},
		{"ssh: connect to host x port 22: Connection timed out", OutcomeNetwork},
		{"ssh: Could not resolve hostname foo: Name or service not known", OutcomeNetwork},
		{"ssh: connect to host x port 22: No route to host", OutcomeNetwork},
		{"some other 255 message", OutcomeNetwork},
	}
	for _, c := range cases {
		if got := classifySSHError(c.msg); got != c.want {
			t.Errorf("classifySSHError(%q) = %q, want %q", c.msg, got, c.want)
		}
	}
}

func TestIsSignalExit(t *testing.T) {
	cases := []struct {
		code int
		want bool
	}{
		{0, false},
		{1, false},
		{127, false},
		{128, true},
		{130, true},
		{159, true},
		{160, false},
		{255, false},
	}
	for _, c := range cases {
		if got := isSignalExit(c.code); got != c.want {
			t.Errorf("isSignalExit(%d) = %v, want %v", c.code, got, c.want)
		}
	}
}
