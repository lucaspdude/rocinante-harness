package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/lucaspdude/rocinante-harness/apps/harness/internal/envconfig"
	"github.com/lucaspdude/rocinante-harness/apps/harness/internal/runner"
)

func runStatus(args []string) error {
	fs := flag.NewFlagSet("status", flag.ExitOnError)
	_ = fs.Parse(args)

	cacheDir := envconfig.CacheDir()
	state, err := runner.LoadState(cacheDir)
	if err != nil {
		fmt.Println("no harness state")
		os.Exit(1)
	}
	apiAlive := processAlive(state.APIPID)
	webAlive := processAlive(state.WebPID)
	uptime := time.Since(state.StartedAt).Round(time.Second)
	apiStr := "down"
	if apiAlive {
		apiStr = fmt.Sprintf("up (pid=%d)", state.APIPID)
	}
	webStr := "down"
	if webAlive {
		webStr = fmt.Sprintf("up (pid=%d)", state.WebPID)
	}
	fmt.Printf("api: %s / web: %s / uptime: %s\n", apiStr, webStr, uptime)
	if !apiAlive || !webAlive {
		os.Exit(3)
	}
	return nil
}
