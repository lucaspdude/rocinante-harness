package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/lucaspdude/rocinante-harness/apps/api/internal/health"
	"github.com/lucaspdude/rocinante-harness/apps/api/internal/omp"
)

const apiVersion = "0.1.0"

// staticMetaLoader returns the cached omp_bin and the handshake
// protocol/omp_version resolved at startup. If the binary is
// missing, all fields are empty and /api/v1/meta returns 503.
type staticMetaLoader struct {
	ompBin          string
	protocolVersion int
	ompVersion      string
}

func (s staticMetaLoader) OmpBin() string { return s.ompBin }
func (s staticMetaLoader) OmpVersion() (int, string) {
	return s.protocolVersion, s.ompVersion
}

func main() {
	port := flag.Int("port", 30179, "HTTP port")
	ompBin := flag.String("omp-bin", "", "path to the omp binary")
	passphraseEnv := flag.String("passphrase-env", "", "env var name for the passphrase")
	noEncryption := flag.Bool("no-encryption", false, "store Ed25519 key in plaintext (dev only)")
	shareDir := flag.String("share-dir", "", "base directory for key/db/logs")
	showVersion := flag.Bool("version", false, "print version and exit")

	flag.Parse()

	if *showVersion {
		fmt.Println("api " + apiVersion)
		return
	}

	loader, cleanup := resolveOmp(*ompBin)
	defer cleanup()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/healthz", health.Handler)
	mux.HandleFunc("/api/v1/meta", omp.NewMetaHandler(loader, apiVersion))

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

	log.Printf("api %s listening on 127.0.0.1:%d (omp-bin=%q share-dir=%q no-encryption=%v)",
		apiVersion, *port, loader.OmpBin(), *shareDir, *noEncryption)

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}

	_ = passphraseEnv
	_ = os.Exit
}

func resolveOmp(flag string) (staticMetaLoader, func()) {
	bin, err := omp.ResolveOmpBin(flag)
	if err != nil {
		return staticMetaLoader{}, func() {}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	sess, err := omp.Spawn(ctx, omp.Options{OpBin: bin})
	if err != nil {
		return staticMetaLoader{ompBin: bin}, func() {}
	}
	proto, ver := sess.Version()
	return staticMetaLoader{
		ompBin:          bin,
		protocolVersion: proto,
		ompVersion:      ver,
	}, func() { _ = sess.Close() }
}
