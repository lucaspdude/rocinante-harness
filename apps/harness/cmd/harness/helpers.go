package main

import (
	"os"
	"syscall"
)

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

func terminateByPid(pid int, sig syscall.Signal) error {
	if pid <= 0 {
		return nil
	}
	proc, err := osFindProcess(pid)
	if err != nil {
		return err
	}
	return proc.Signal(sig)
}

func osFindProcess(pid int) (*os.Process, error) {
	return os.FindProcess(pid)
}
