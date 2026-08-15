package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/lucaspdude/rocinante-harness/apps/api/internal/api"
	"github.com/lucaspdude/rocinante-harness/apps/api/internal/api/middleware"
	"github.com/lucaspdude/rocinante-harness/apps/api/internal/omp"
)

const apiVersion = "0.1.0"

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
	noEncryption := flag.Bool("no-encryption", false, "store Ed25519 key in plaintext (dev only)")
	shareDir := flag.String("share-dir", "", "base directory for key/db/logs")
	showVersion := flag.Bool("version", false, "print version and exit")

	flag.Parse()

	if *showVersion {
		fmt.Println("api " + apiVersion)
		return
	}

	loader, resolvedBin := resolveOmp(*ompBin)
	manager := omp.NewManager(resolvedBin)
	idem := middleware.NewIdempotencyCache(2048)

	mux := http.NewServeMux()
	mux.Handle("/", api.NewRouter(api.RouterDeps{
		MetaLoader:  loader,
		Manager:     manager,
		APIVersion:  apiVersion,
		Idempotency: idem,
	}))

	srv := &http.Server{
		Addr:              fmt.Sprintf("127.0.0.1:%d", *port),
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()
		manager.CloseAll()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	log.Printf("api %s listening on 127.0.0.1:%d (omp-bin=%q share-dir=%q no-encryption=%v)",
		apiVersion, *port, resolvedBin, *shareDir, *noEncryption)

	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("server error: %v", err)
	}
}

func resolveOmp(flag string) (staticMetaLoader, string) {
	bin, err := omp.ResolveOmpBin(flag)
	if err != nil {
		return staticMetaLoader{}, ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	sess, err := omp.Spawn(ctx, omp.Options{OpBin: bin})
	if err != nil {
		return staticMetaLoader{ompBin: bin}, bin
	}
	proto, ver := sess.Version()
	_ = sess.Close()
	return staticMetaLoader{
		ompBin:          bin,
		protocolVersion: proto,
		ompVersion:      ver,
	}, bin
}
