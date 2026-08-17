// Package files holds the file access allow-list and the API
// handlers that gate read-only filesystem access. PR-07 wires the
// HTTP routes; this module owns the path resolution + safety
// invariants used by every file endpoint.
package files

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// FileAccess holds the set of paths the api is allowed to serve.
// PR-03 seeds it via ProjectRegistry + session-spawn. The set is
// in-memory only; persistent storage goes via <share-dir>/.
type FileAccess struct {
	mu    sync.RWMutex
	roots map[string]string // canonical -> original (display)
}

// ErrPathOutsideAllowList is returned when a path resolves outside
// any allow-listed root.
var ErrPathOutsideAllowList = errors.New("path_outside_allowlist")

// ErrEmptyPath is returned when the caller passes "".
var ErrEmptyPath = errors.New("empty_path")

// NewFileAccess returns an empty allow-list.
func NewFileAccess() *FileAccess {
	return &FileAccess{roots: make(map[string]string)}
}

// Allow adds the given root. Symlinks are resolved before
// storage. If the path doesn't exist, the symlink resolution may
// fail; we accept the original path as a fall-back (rare in
// practice; future PR can refuse).
func (a *FileAccess) Allow(root string) error {
	if root == "" {
		return ErrEmptyPath
	}
	canonical, err := filepath.EvalSymlinks(root)
	if err != nil {
		// Use the original path even if it doesn't resolve.
		canonical = filepath.Clean(root)
	}
	a.mu.Lock()
	a.roots[canonical] = root
	a.mu.Unlock()
	return nil
}

// Disallow removes a previously-allowed root. No error if absent.
func (a *FileAccess) Disallow(root string) {
	canonical, _ := filepath.EvalSymlinks(root)
	if canonical == "" {
		canonical = filepath.Clean(root)
	}
	a.mu.Lock()
	delete(a.roots, canonical)
	delete(a.roots, filepath.Clean(root))
	a.mu.Unlock()
}

// IsAllowed reports whether the given path resolves under any
// allow-listed root. Symlinks are followed via realpath.
func (a *FileAccess) IsAllowed(p string) bool {
	if p == "" {
		return false
	}
	canonical, err := filepath.EvalSymlinks(p)
	if err != nil {
		canonical = filepath.Clean(p)
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	for root := range a.roots {
		if pathHasPrefix(canonical, root) {
			return true
		}
	}
	// Fall back to also testing the original root strings, in case
	// symlink resolution varies between calls.
	for _, original := range a.roots {
		cleanOrig := filepath.Clean(original)
		if pathHasPrefix(canonical, cleanOrig) || pathHasPrefix(filepath.Clean(p), cleanOrig) {
			return true
		}
	}
	return false
}

// Roots returns a stable, sorted list of currently-allow-listed roots.
func (a *FileAccess) Roots() []string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	out := make([]string, 0, len(a.roots))
	for root := range a.roots {
		out = append(out, root)
	}
	sort.Strings(out)
	return out
}

// pathHasPrefix is filepath.HasPrefix without the case-folding
// weirdness; we treat all paths on a single host as one case
// (mac/linux on APFS is path-preserving).
func pathHasPrefix(p, prefix string) bool {
	if prefix == "" || prefix == "/" {
		return true
	}
	p = filepath.Clean(p)
	prefix = filepath.Clean(prefix)
	if p == prefix {
		return true
	}
	if !strings.HasSuffix(prefix, string(filepath.Separator)) {
		prefix += string(filepath.Separator)
	}
	return strings.HasPrefix(p, prefix)
}

// Resolve is a helper that returns the resolved absolute path under
// a given root, or an error if the resolution escapes the root.
// Used by the file handlers (PR-07).
func Resolve(root, rel string) (string, error) {
	if root == "" {
		return "", ErrEmptyPath
	}
	canonicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		canonicalRoot = filepath.Clean(root)
	}
	joined := filepath.Join(canonicalRoot, rel)
	canonical, err := filepath.EvalSymlinks(joined)
	if err != nil {
		// Path doesn't exist (yet). Use the joined path to compute
		// the check; this allows read-only fs browsing of dirs even
		// when the leaf doesn't exist (e.g. a non-existent inner
		// path will fail with file-not-found later, which is the
		// correct error to surface).
		canonical = filepath.Clean(joined)
	}
	if !pathHasPrefix(canonical, canonicalRoot) {
		return canonical, ErrPathOutsideAllowList
	}
	return canonical, nil
}

// QuietAllow is a convenience for tests / non-critical paths that
// silently no-op on error.
func (a *FileAccess) QuietAllow(root string) {
	_ = a.Allow(root)
}

// QuietDisallow is a convenience for cleanup paths.
func (a *FileAccess) QuietDisallow(root string) {
	a.Disallow(root)
}

// silence unused import warnings in build slices that drop os.
var _ = os.PathSeparator
