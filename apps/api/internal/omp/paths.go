// Package omp wraps the stdio RPC interface to the omp binary.
package omp

import (
	"errors"
	"os"
	"os/exec"
)

// ErrOmpNotFound is returned when no usable omp binary is found.
var ErrOmpNotFound = errors.New("omp binary not found")

// ResolveOmpBin follows the lookup order: explicit flag, $OMP_BIN,
// $PATH. Lookup in $PATH returns the absolute path on success, or
// ErrOmpNotFound without a fatal. An empty result is preserved.
func ResolveOmpBin(flag string) (string, error) {
	if flag != "" {
		return flag, nil
	}
	if env := os.Getenv("OMP_BIN"); env != "" {
		return env, nil
	}
	path, err := exec.LookPath("omp")
	if err != nil {
		return "", ErrOmpNotFound
	}
	return path, nil
}
