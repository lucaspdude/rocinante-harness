package main

import (
	"flag"
	"fmt"
	"path/filepath"

	"github.com/lucaspdude/rocinante-harness/apps/harness/internal/envconfig"
	"github.com/lucaspdude/rocinante-harness/apps/harness/internal/runner"
)

func runLogs(args []string) error {
	fs := flag.NewFlagSet("logs", flag.ExitOnError)
	tail := fs.Int("tail", 100, "number of lines to print")
	follow := fs.Bool("follow", false, "follow the log (tail -f style)")
	_ = fs.Parse(args)

	shareDir := envconfig.ShareDir()
	logDir := filepath.Join(shareDir, "logs")
	if !*follow {
		out, err := runner.TailLogs(logDir, *tail)
		if err != nil {
			return err
		}
		fmt.Print(out)
		return nil
	}
	out, err := runner.TailLogs(logDir, *tail)
	if err != nil {
		return err
	}
	fmt.Print(out)
	fmt.Println("--- (follow mode: re-run to refresh) ---")
	return nil
}
