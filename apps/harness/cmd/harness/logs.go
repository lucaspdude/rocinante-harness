package main

import (
	"flag"
	"fmt"

	"github.com/lucaspdude/rocinante-harness/apps/harness/internal/runner"
)

func runLogs(args []string) error {
	fs := flag.NewFlagSet("logs", flag.ExitOnError)
	tail := fs.Int("tail", 100, "number of lines to print")
	follow := fs.Bool("follow", false, "follow the log (tail -f style)")
	_ = fs.Parse(args)

	shareDir := defaultShareDir()
	logDir := shareDir + "/logs"
	if !*follow {
		out, err := runner.TailLogs(logDir, *tail)
		if err != nil {
			return err
		}
		fmt.Print(out)
		return nil
	}
	// Follow mode: just print the last tail and exit (proper
	// tail-f is out of scope for this MVP).
	out, err := runner.TailLogs(logDir, *tail)
	if err != nil {
		return err
	}
	fmt.Print(out)
	fmt.Println("--- (follow mode: re-run to refresh) ---")
	return nil
}
