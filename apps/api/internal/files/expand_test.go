package files

import (
	"path/filepath"
	"testing"
)

func TestExpandHome(t *testing.T) {
	home := "/root"
	cases := []struct {
		in, want string
	}{
		{"~", "/root"},
		{"~/", "/root"},
		{"~/projects", "/root/projects"},
		{"~/projects/my-app", "/root/projects/my-app"},
		// Already absolute — passes through.
		{"/srv/foo", "/srv/foo"},
		// Relative paths (not starting with ~) pass through.
		{"foo", "foo"},
		{"./bar", "./bar"},
		// Tilde in the middle is NOT a leading tilde.
		{"/srv/~user/foo", "/srv/~user/foo"},
		// Empty stays empty (caller validates).
		{"", ""},
	}
	for _, c := range cases {
		got := ExpandHome(c.in, home)
		if got != c.want {
			t.Errorf("ExpandHome(%q,%q) = %q, want %q", c.in, home, got, c.want)
		}
	}
}

func TestExpandHomeJoinsCleanly(t *testing.T) {
	// filepath.Join collapses separators; verify the trailing-slash
	// and nested cases produce a clean absolute path.
	got := ExpandHome("~/a/b", "/root")
	want := filepath.Join("/root", "a", "b")
	if got != want {
		t.Errorf("ExpandHome(~/a/b) = %q, want %q", got, want)
	}
}
