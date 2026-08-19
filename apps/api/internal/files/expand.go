package files

import (
	"path/filepath"
	"strings"
)

// ExpandHome resolves "~" and "~/..." to an absolute path under
// the supplied home directory. Other paths pass through untouched
// (already-absolute or relative). Used by handlers that accept
// user-supplied paths from the picker so users can type a tilde
// instead of an absolute path.
func ExpandHome(p, home string) string {
	if p == "~" {
		return home
	}
	if strings.HasPrefix(p, "~/") {
		return filepath.Join(home, p[2:])
	}
	return p
}
