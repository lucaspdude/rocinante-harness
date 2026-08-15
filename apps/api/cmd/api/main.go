package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/lucaspdude/rocinante-harness/apps/api/internal/api"
	"github.com/lucaspdude/rocinante-harness/apps/api/internal/api/middleware"
	"github.com/lucaspdude/rocinante-harness/apps/api/internal/omp"
	"github.com/lucaspdude/rocinante-harness/apps/api/internal/storage"
)

const apiVersion = "0.1.0"

func defaultShareDir() string {
	if v := os.Getenv("ROCHASSEN_SHARE_DIR"); v != "" {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "/tmp/rocinante-harness"
	}
	return filepath.Join(home, ".local", "share", "rocinante-harness")
}

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
	shareDir := flag.String("share-dir", "", "base directory for key/db/logs (default: ROCHASSEN_SHARE_DIR or ~/.local/share/rocinante-harness)")
	passphraseEnv := flag.String("passphrase-env", "", "env var name for the passphrase")
	dbPath := flag.String("db-path", "", "deprecated: alias for --share-dir/roc-harness.db")
	showVersion := flag.Bool("version", false, "print version and exit")

	flag.Parse()

	if *showVersion {
		fmt.Println("api " + apiVersion)
		return
	}

	if len(flag.Args()) > 0 && flag.Args()[0] == "init" {
		dir := *shareDir
		if dir == "" {
			dir = defaultShareDir()
		}
		if err := initShare(dir, *noEncryption, *passphraseEnv); err != nil {
			log.Fatalf("init: %v", err)
		}
		return
	}

	effectiveShareDir := *shareDir
	if effectiveShareDir == "" {
		effectiveShareDir = defaultShareDir()
	}
	if *dbPath != "" {
		log.Printf("warning: --db-path is deprecated; use --share-dir instead")
	}

	loader, resolvedBin := resolveOmp(*ompBin)
	manager := omp.NewManager(resolvedBin)
	idem := middleware.NewIdempotencyCache(2048)

	// Open SQLite (best-effort — api runs without DB if init hasn't been run).
	dbPathResolved := filepath.Join(effectiveShareDir, "roc-harness.db")
	db, err := storage.Open(dbPathResolved)
	if err != nil {
		log.Printf("warning: storage open failed (%v); continuing without DB", err)
		db = nil
	} else {
		if err := storage.ApplyMigrations(db); err != nil {
			log.Printf("warning: migrations failed: %v", err)
		}
		_ = db
	}

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
		apiVersion, *port, resolvedBin, effectiveShareDir, *noEncryption)

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
