package omp

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveOmpBinFlagWins(t *testing.T) {
	t.Setenv("OMP_BIN", "/should/not/be/used")
	got, err := ResolveOmpBin("/explicit/omp")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got != "/explicit/omp" {
		t.Errorf("got %q, want /explicit/omp", got)
	}
}

func TestResolveOmpBinEnvWins(t *testing.T) {
	t.Setenv("OMP_BIN", "/from/env/omp")
	got, err := ResolveOmpBin("")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got != "/from/env/omp" {
		t.Errorf("got %q, want /from/env/omp", got)
	}
}

func TestResolveOmpBinPathLookup(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "omp")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write: %v", err)
	}
	t.Setenv("OMP_BIN", "")
	t.Setenv("PATH", dir)
	got, err := ResolveOmpBin("")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got != bin {
		t.Errorf("got %q, want %q", got, bin)
	}
}

func TestResolveOmpBinNotFound(t *testing.T) {
	t.Setenv("OMP_BIN", "")
	t.Setenv("PATH", t.TempDir())
	_, err := ResolveOmpBin("")
	if err != ErrOmpNotFound {
		t.Errorf("err = %v, want ErrOmpNotFound", err)
	}
}
