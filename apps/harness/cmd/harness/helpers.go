package main

import (
	"os"
	"path/filepath"
	"runtime"
	"syscall"
)

func defaultShareDir() string {
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

func defaultCacheDir() string {
	if v := os.Getenv("ROCHASSEN_CACHE_DIR"); v != "" {
		return v
	}
	if runtime.GOOS == "windows" {
		return defaultShareDir() + "\\cache"
	}
	return filepath.Join(defaultShareDir(), "cache")
}

func defaultAPIBin() string {
	if v := os.Getenv("RH_API_BIN"); v != "" {
		return v
	}
	return filepath.Join(defaultShareDir(), "bin", "api")
}

func defaultWebDir() string {
	if v := os.Getenv("RH_WEB_DIR"); v != "" {
		return v
	}
	return filepath.Join(defaultShareDir(), "web")
}

func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}

func osFindProcess(pid int) (*os.Process, error) {
	return os.FindProcess(pid)
}
