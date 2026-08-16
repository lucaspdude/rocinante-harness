package main

import (
	"flag"
	"log"
	"syscall"

	"github.com/lucaspdude/rocinante-harness/apps/harness/internal/envconfig"
	"github.com/lucaspdude/rocinante-harness/apps/harness/internal/runner"
)

func runDown(args []string) error {
	fs := flag.NewFlagSet("down", flag.ExitOnError)
	_ = fs.Parse(args)

	cacheDir := envconfig.CacheDir()
	state, err := runner.LoadState(cacheDir)
	if err != nil {
		log.Printf("no state file found at %s", cacheDir)
		return nil
	}
	log.Printf("loaded state: api=%d web=%d", state.APIPID, state.WebPID)
	if err := terminateByPid(state.APIPID, syscall.SIGTERM); err != nil {
		log.Printf("api: %v", err)
	}
	if err := terminateByPid(state.WebPID, syscall.SIGTERM); err != nil {
		log.Printf("web: %v", err)
	}
	_ = runner.New(cacheDir, "").StopAll()
	return nil
}
