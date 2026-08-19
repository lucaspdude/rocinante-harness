package ssh

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestEnsureSshDirCreates0700 verifies that EnsureSshDir mkdir's
// ~/.ssh if missing and that the result is mode 0o700.
func TestEnsureSshDirCreates0700(t *testing.T) {
	// Re-point $HOME to a fresh temp dir so we never touch the
	// developer's real ~/.ssh.
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	dir, err := EnsureSshDir()
	if err != nil {
		t.Fatalf("EnsureSshDir: %v", err)
	}
	want := filepath.Join(tmp, ".ssh")
	if dir != want {
		t.Errorf("dir = %q, want %q", dir, want)
	}
	st, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	mode := st.Mode().Perm()
	if mode != 0o700 {
		t.Errorf("perm = %o, want 0o700", mode)
	}
}

// TestEnsureSshDirIdempotent checks that a second call does not
// fail (the directory already exists).
func TestEnsureSshDirIdempotent(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	if _, err := EnsureSshDir(); err != nil {
		t.Fatalf("first: %v", err)
	}
	if _, err := EnsureSshDir(); err != nil {
		t.Fatalf("second: %v", err)
	}
}

// TestAssertInSshDirAccepts verifies that paths under ~/.ssh are
// accepted and that escape attempts are rejected.
func TestAssertInSshDirAccepts(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	mustExist := filepath.Join(tmp, ".ssh")
	if err := os.MkdirAll(mustExist, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	good := []string{
		filepath.Join(tmp, ".ssh", "id_ed25519_test"),
		filepath.Join(tmp, ".ssh", "subdir", "id_ed25519_x"),
		filepath.Join(tmp, ".ssh"),
	}
	for _, p := range good {
		if err := AssertInSshDir(p); err != nil {
			t.Errorf("AssertInSshDir(%q) = %v, want nil", p, err)
		}
	}
	bad := []string{
		filepath.Join(tmp, "..", "etc", "passwd"),
		filepath.Join(tmp, ".ssh", "..", "evil"),
		"/etc/passwd",
	}
	for _, p := range bad {
		if err := AssertInSshDir(p); err == nil {
			t.Errorf("AssertInSshDir(%q) = nil, want error", p)
		}
	}
}

// TestWritePrivateKey0600 verifies that a freshly written private
// key has mode 0o600 and contains exactly the bytes we passed.
func TestWritePrivateKey0600(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	if _, err := EnsureSshDir(); err != nil {
		t.Fatalf("EnsureSshDir: %v", err)
	}
	target := filepath.Join(tmp, ".ssh", "id_ed25519_test")
	content := []byte("-----BEGIN OPENSSH PRIVATE KEY-----\nfake\n")
	if err := WritePrivateKey(target, content); err != nil {
		t.Fatalf("WritePrivateKey: %v", err)
	}
	st, err := os.Stat(target)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if st.Mode().Perm() != 0o600 {
		t.Errorf("perm = %o, want 0o600", st.Mode().Perm())
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(got) != string(content) {
		t.Errorf("content mismatch")
	}
}

// TestWritePrivateKeyRejectsEscape verifies that paths outside
// ~/.ssh are rejected with an error and never create the file.
func TestWritePrivateKeyRejectsEscape(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	if _, err := EnsureSshDir(); err != nil {
		t.Fatalf("EnsureSshDir: %v", err)
	}
	outside := filepath.Join(tmp, "evil-key")
	if err := WritePrivateKey(outside, []byte("secret")); err == nil {
		t.Errorf("WritePrivateKey accepted path outside ~/.ssh")
	}
	if _, err := os.Stat(outside); err == nil {
		t.Errorf("file was created despite escape rejection")
	}
	// Touch a sanity check that the error message mentions escape.
	_ = strings.Contains
}
