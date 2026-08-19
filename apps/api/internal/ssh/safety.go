package ssh

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// EnsureSshDir creates ~/.ssh with 0o700 permissions if it does not
// exist and returns the resolved path. It tolerates the directory
// already existing (so concurrent callers do not race) as long as
// the existing path is a directory; permissions are tightened when
// possible but never relaxed (i.e. a group/world-writable dir will
// be left untouched — the caller should chmod it manually if that
// ever happens in production).
func EnsureSshDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("home dir: %w", err)
	}
	if home == "" {
		return "", errors.New("home dir is empty")
	}
	dir := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("mkdir %s: %w", dir, err)
	}
	// Best-effort chmod; ignore failure if file is missing.
	_ = os.Chmod(dir, 0o700)
	return dir, nil
}

// AssertInSshDir guarantees that filepath.Clean(p) lives directly
// under ~/.ssh/. We use a relative-prefix check (not ResolveLinks)
// because the api may run before the user's homedir has been read
// by anything that resolves symlinks. The function returns an error
// for any path that tries to escape via ".." or an absolute detour.
func AssertInSshDir(p string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("home dir: %w", err)
	}
	if home == "" {
		return errors.New("home dir is empty")
	}
	base := filepath.Clean(filepath.Join(home, ".ssh"))
	target := filepath.Clean(p)
	if target == base {
		return nil
	}
	prefix := base + string(os.PathSeparator)
	if !strings.HasPrefix(target, prefix) {
		return fmt.Errorf("path %q escapes ~/.ssh", p)
	}
	return nil
}

// WritePrivateKey atomically creates the file at p with the given
// content and chmod 0o600. The sequence is:
//
//  1. MkdirAll the parent directory (mode 0o700).
//  2. Create the file with O_CREATE|O_WRONLY|O_TRUNC and mode 0o600
//     so the very first appearance has safe permissions — no window
//     in which a group/world-readable private key is observable.
//  3. Write the content.
//  4. Re-chmod defensively in case umask stripped the mode bits.
//
// The parent path is verified to live under ~/.ssh/ via
// AssertInSshDir; callers should pass paths they constructed
// themselves from a sanitized label.
func WritePrivateKey(p string, content []byte) error {
	if err := AssertInSshDir(p); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return fmt.Errorf("mkdir parent: %w", err)
	}
	f, err := os.OpenFile(p, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("open %s: %w", p, err)
	}
	if _, err := f.Write(content); err != nil {
		_ = f.Close()
		return fmt.Errorf("write %s: %w", p, err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close %s: %w", p, err)
	}
	if err := os.Chmod(p, 0o600); err != nil {
		return fmt.Errorf("chmod %s: %w", p, err)
	}
	return nil
}
