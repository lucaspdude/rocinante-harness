// Package envconfig resolves the api's runtime config from the
// environment. Honors both ROCINANTE_* (preferred) and the legacy
// ROCHASSEN_* alias from the early alpha.
package envconfig

import (
	"os"
	"path/filepath"
)

// ShareDir returns the directory used for the key, SQLite DB, and
// logs.
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

// OmpBin returns the path to the omp binary. Empty string means
// "let omp.ResolveOmpBin hunt for it".
func OmpBin() string {
	if v := os.Getenv("ROCINANTE_OMP_BIN"); v != "" {
		return v
	}
	if v := os.Getenv("OMP_BIN"); v != "" {
		return v
	}
	return os.Getenv("ROCHASSEN_OMP_BIN")
}

// PassphraseEnv returns the name of the env var that holds the
// api's passphrase (not the passphrase itself). Empty string
// means "no passphrase configured"; init will then prompt
// interactively or fail.
func PassphraseEnv() string {
	if v := os.Getenv("ROCINANTE_PASSPHRASE_ENV"); v != "" {
		return v
	}
	if v := os.Getenv("ROCHASSEN_PASSPHRASE_ENV"); v != "" {
		return v
	}
	// Legacy: --passphrase-env flag was the previous mechanism.
	// The installer writes ROCINANTE_PASSPHRASE=... and we read it
	// here without a separate env var name.
	if v := os.Getenv("ROCINANTE_PASSPHRASE"); v != "" {
		return "ROCINANTE_PASSPHRASE"
	}
	return ""
}
