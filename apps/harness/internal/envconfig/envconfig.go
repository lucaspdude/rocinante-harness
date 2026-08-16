// Package envconfig resolves the share/cache paths from the
// environment, honouring both ROCINANTE_* (preferred) and
// ROCHASSEN_* (legacy alias from the early alpha).
package envconfig

import (
	"os"
	"path/filepath"
	"runtime"
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
	if runtime.GOOS == "windows" {
		if v := os.Getenv("LOCALAPPDATA"); v != "" {
			return filepath.Join(v, "rocinante-harness")
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "/tmp/rocinante-harness"
	}
	return filepath.Join(home, ".local", "share", "rocinante-harness")
}

// CacheDir returns the directory used for the PID file. Honors
// ROCINANTE_CACHE_DIR then ROCHASSEN_CACHE_DIR.
func CacheDir() string {
	if v := os.Getenv("ROCINANTE_CACHE_DIR"); v != "" {
		return v
	}
	if v := os.Getenv("ROCHASSEN_CACHE_DIR"); v != "" {
		return v
	}
	if runtime.GOOS == "windows" {
		return ShareDir() + "\\cache"
	}
	return filepath.Join(ShareDir(), "cache")
}

// APIBin returns the path to the api binary. Honors
// ROCINANTE_API_BIN then ROCHASSEN_API_BIN then ${share}/bin/api.
func APIBin() string {
	if v := os.Getenv("ROCINANTE_API_BIN"); v != "" {
		return v
	}
	if v := os.Getenv("ROCHASSEN_API_BIN"); v != "" {
		return v
	}
	return filepath.Join(ShareDir(), "bin", "api")
}

// WebDir returns the path to the Next build directory. Honors
// ROCINANTE_WEB_DIR then ROCHASSEN_WEB_DIR then ${share}/web.
func WebDir() string {
	if v := os.Getenv("ROCINANTE_WEB_DIR"); v != "" {
		return v
	}
	if v := os.Getenv("ROCHASSEN_WEB_DIR"); v != "" {
		return v
	}
	return filepath.Join(ShareDir(), "web")
}
