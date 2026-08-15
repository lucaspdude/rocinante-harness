package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/lucaspdude/rocinante-harness/apps/api/internal/health"
)

const apiVersion = "0.1.0"

func main() {
	port := flag.Int("port", 30179, "HTTP port to listen on")
	ompBin := flag.String("omp-bin", "", "path to the omp binary (default: $PATH)")
	passphraseEnv := flag.String("passphrase-env", "", "env var name that holds the passphrase")
	noEncryption := flag.Bool("no-encryption", false, "store the Ed25519 key in plaintext (dev only)")
	shareDir := flag.String("share-dir", "", "base directory for key/db/logs (default: $XDG_DATA_HOME/rocinante-harness)")
	dbPath := flag.String("db-path", "", "deprecated: alias for --share-dir/roc-harness.db")
	ed25519Path := flag.String("ed25519-path", "", "deprecated: alias for --share-dir/.ed25519")
	showVersion := flag.Bool("version", false, "print version and exit")

	flag.Parse()

	if *showVersion {
		fmt.Println("api " + apiVersion)
		return
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/healthz", health.Handler)

	srv := &http.Server{
		Addr:              fmt.Sprintf("127.0.0.1:%d", *port),
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	log.Printf("api 0.1.0 listening on 127.0.0.1:%d (omp-bin=%q share-dir=%q no-encryption=%v)",
		*port, *ompBin, *shareDir, *noEncryption)

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}

	_ = json.Marshal
	_ = passphraseEnv
	_ = dbPath
	_ = ed25519Path
	_ = os.Exit
}
