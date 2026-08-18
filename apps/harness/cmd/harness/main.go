package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

const harnessVersion = "1.0.0"

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
		fmt.Println("harness " + harnessVersion)
		return
	}
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}
	switch os.Args[1] {
	case "up":
		if err := runUp(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "up:", err)
			os.Exit(1)
		}
	case "down":
		if err := runDown(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "down:", err)
			os.Exit(1)
		}
	case "status":
		if err := runStatus(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "status:", err)
			os.Exit(1)
		}
	case "logs":
		if err := runLogs(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "logs:", err)
			os.Exit(1)
		}
	case "version":
		fmt.Println("harness " + harnessVersion)
	default:
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "harness up|down|status|logs|version")
}

// notifyOnSignal returns a channel that closes on SIGINT/SIGTERM.
func notifyOnSignal() chan struct{} {
	notif := make(chan struct{})
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig
		close(notif)
	}()
	return notif
}
