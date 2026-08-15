package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"path/filepath"
	"time"

	"github.com/lucaspdude/rocinante-harness/apps/harness/internal/runner"
)

func runUp(args []string) error {
	fs := flag.NewFlagSet("up", flag.ExitOnError)
	apiBin := fs.String("api-bin", defaultAPIBin(), "path to the api binary")
	webDir := fs.String("web-dir", defaultWebDir(), "path to the Next build output")
	apiPort := fs.Int("api-port", 30179, "HTTP port for the api")
	webPort := fs.Int("web-port", 30178, "HTTP port for the web")
	shareDir := fs.String("share-dir", defaultShareDir(), "directory for key/db/logs")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cacheDir := defaultCacheDir()
	logDir := filepath.Join(*shareDir, "logs")
	r := runner.New(cacheDir, logDir)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log.Printf("starting api (bin=%s port=%d share-dir=%s)", *apiBin, *apiPort, *shareDir)
	if err := r.StartAPI(ctx, *apiBin, *apiPort, *shareDir); err != nil {
		return fmt.Errorf("start api: %w", err)
	}
	if err := waitForPort(*apiPort, 30*time.Second); err != nil {
		_ = r.StopAll()
		return fmt.Errorf("api never became ready: %w", err)
	}
	log.Printf("starting web (dir=%s port=%d)", *webDir, *webPort)
	if err := r.StartWeb(ctx, *webDir, *webPort); err != nil {
		_ = r.StopAll()
		return fmt.Errorf("start web: %w", err)
	}
	if err := waitForPort(*webPort, 30*time.Second); err != nil {
		_ = r.StopAll()
		return fmt.Errorf("web never became ready: %w", err)
	}
	if err := r.PIDFile(); err != nil {
		log.Printf("warning: write pid file: %v", err)
	}

	log.Printf("api up (pid=%d) / web up (pid=%d). ctrl-c to stop.",
		r.State().APIPID, r.State().WebPID)

	notif := notifyOnSignal()
	<-notif
	log.Printf("stopping children...")
	return r.StopAll()
}

func waitForPort(port int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 200*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("port %d not reachable", port)
}
