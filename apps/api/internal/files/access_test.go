package files

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAllowAndIsAllowed(t *testing.T) {
	dir := t.TempDir()
	fa := NewFileAccess()
	if err := fa.Allow(dir); err != nil {
		t.Fatalf("Allow: %v", err)
	}
	if !fa.IsAllowed(dir) {
		t.Error("root should be allowed")
	}
	if !fa.IsAllowed(filepath.Join(dir, "sub", "file.txt")) {
		t.Error("nested file under allow should be allowed")
	}
	outside := filepath.Join(dir, "..", "outside.txt")
	if fa.IsAllowed(outside) {
		t.Error("path that escapes via .. should be rejected")
	}
}

func TestAllowSymlinkBypass(t *testing.T) {
	dir := t.TempDir()
	outside := t.TempDir()
	outsideFile := filepath.Join(outside, "secret.txt")
	if err := os.WriteFile(outsideFile, []byte("nope"), 0o600); err != nil {
		t.Fatal(err)
	}
	fa := NewFileAccess()
	if err := fa.Allow(dir); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "leak")
	if err := os.Symlink(outsideFile, link); err != nil {
		t.Skip("symlinks not supported in this environment")
	}
	canonical, err := filepath.EvalSymlinks(link)
	if err != nil {
		t.Fatal(err)
	}
	if fa.IsAllowed(canonical) {
		t.Errorf("symlink-bypass should be blocked (resolved=%q, allowed=%+v)",
			canonical, fa.Roots())
	}
}

func TestResolveRejectOutsideRoot(t *testing.T) {
	dir := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "x.txt"), []byte("z"), 0o600); err != nil {
		t.Fatal(err)
	}
	fa := NewFileAccess()
	_ = fa.Allow(dir)
	_, err := Resolve(dir, "../escape.txt")
	if err == nil {
		t.Error("expected ErrPathOutsideAllowList")
	}
}

func TestResolveAcceptsSubpath(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "ok.txt"), []byte("ok"), 0o600); err != nil {
		t.Fatal(err)
	}
	fa := NewFileAccess()
	_ = fa.Allow(dir)
	resolved, err := Resolve(dir, "ok.txt")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if !fa.IsAllowed(resolved) {
		t.Errorf("resolved path should be allowed; got %q", resolved)
	}
}

func TestDisallow(t *testing.T) {
	dir := t.TempDir()
	fa := NewFileAccess()
	if err := fa.Allow(dir); err != nil {
		t.Fatal(err)
	}
	fa.QuietDisallow(dir)
	if fa.IsAllowed(dir) {
		t.Error("dir should be disallowed")
	}
}

func TestRootsSorted(t *testing.T) {
	fa := NewFileAccess()
	_ = fa.Allow("/tmp/b")
	_ = fa.Allow("/tmp/a")
	_ = fa.Allow("/tmp/c")
	got := fa.Roots()
	if len(got) != 3 || got[0] != "/tmp/a" || got[1] != "/tmp/b" || got[2] != "/tmp/c" {
		t.Errorf("Roots = %+v", got)
	}
}
