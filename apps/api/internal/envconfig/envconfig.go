// Package envconfig resolves the share/cache paths from the
// environment, honouring both ROCINANTE_* (preferred) and
// ROCHASSEN_* (legacy alias from the early alpha).
package envconfig

import (
	"os"
	"path/filepath"
)

// ShareDir returns the directory used for the key, SQLite DB, and
// logs. Honors ROCINANTE_SHARE_DIR then ROCHASSEN_SHARE_DIR.
func ShareDir() string {
	if v := os.Getenv("ROCINANTE_SHARE_DIR"); v != "" {
		return v
	}
	if v := os.Getenv("ROCHASSEN_SHARE_DIR"); v != "" {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "/tmp/rocinante-harness"
	}
	return filepath.Join(home, ".local", "share", "rocinante-harness")
}
