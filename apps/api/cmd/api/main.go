package main

import (
	"context"
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/lucaspdude/rocinante-harness/apps/api/internal/api"
	"github.com/lucaspdude/rocinante-harness/apps/api/internal/api/middleware"
	"github.com/lucaspdude/rocinante-harness/apps/api/internal/auth"
	"github.com/lucaspdude/rocinante-harness/apps/api/internal/omp"
	sshpkg "github.com/lucaspdude/rocinante-harness/apps/api/internal/ssh"
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
	shareDir := flag.String("share-dir", "", "base directory for key/db/logs")
	passphraseEnv := flag.String("passphrase-env", "", "env var name for the passphrase")
	bind := flag.String("bind", "127.0.0.1", "bind address (127.0.0.1 | 0.0.0.0)")
	corsAllowlist := flag.String("cors-allowlist", "", "comma-separated allowed Origin values (required when --bind is 0.0.0.0)")
	tlsCert := flag.String("tls-cert", "", "TLS certificate path (PEM)")
	tlsKey := flag.String("tls-key", "", "TLS private key path (PEM)")
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

	loader, resolvedBin := resolveOmp(*ompBin)
	manager := omp.NewManager(resolvedBin)
	idem := middleware.NewIdempotencyCache(2048)

	if *bind == "0.0.0.0" && *corsAllowlist == "" {
		log.Fatalf("--bind 0.0.0.0 requires --cors-allowlist (defense-in-depth)")
	}

	dbPathResolved := filepath.Join(effectiveShareDir, "roc-harness.db")
	db, dbErr := storage.Open(dbPathResolved)
	if dbErr != nil {
		log.Printf("warning: storage open failed (%v); continuing without DB", dbErr)
	}

	var (
		authState *api.AuthState
		authMW    func(http.Handler) http.Handler
	)
	if dbErr == nil {
		if err := storage.ApplyMigrations(db); err != nil {
			log.Printf("warning: migrations failed: %v", err)
		}
		ed25519Path := filepath.Join(effectiveShareDir, ".ed25519")
		passphrase := ""
		if *passphraseEnv != "" {
			passphrase = os.Getenv(*passphraseEnv)
		}
		sk, pk, err := auth.LoadKeyFile(ed25519Path, passphrase)
		if err != nil {
			log.Printf("warning: cannot load %s (%v); auth endpoints disabled", ed25519Path, err)
		} else {
			signer := auth.NewSigner(sk, pk, passphrase)
			authState = &api.AuthState{
				Signer:       signer,
				RefreshStore: auth.NewRefreshStore(db),
				DeviceStore:  auth.NewDeviceStore(db),
				PairingStore: auth.NewPairingStore(db),
			}
			revCache := auth.NewRevocationCache(db, &auth.StaticKeyLoader{Pk: pk})
			defer revCache.Stop()
			authMW = auth.AuthMiddleware(&auth.StaticKeyLoader{Pk: pk}, revCache)
		}
		defer db.Close()
	}

	var corsOrigins []string
	if *corsAllowlist != "" {
		for _, o := range strings.Split(*corsAllowlist, ",") {
			corsOrigins = append(corsOrigins, strings.TrimSpace(o))
		}
	}

	mux := http.NewServeMux()
	mux.Handle("/", middleware.TLSHandler(
		middleware.CORSHandler(middleware.CORSConfig{AllowedOrigins: corsOrigins})(
			api.NewRouter(api.RouterDeps{
				MetaLoader:  loader,
				Manager:     manager,
				APIVersion:  apiVersion,
				Idempotency: idem,
				AuthState:   authState,
				AuthMW:      authMW,
				ShareDir:    effectiveShareDir,
			}),
		),
	))

	if dbErr == nil && authMW != nil {
		sshHandler := &sshpkg.Handler{
			Keys:    sshpkg.NewKeyStore(db),
			Servers: sshpkg.NewServerStore(db),
			AuthMW:  authMW,
		}
		mux.Handle("/api/v1/ssh/", middleware.TLSHandler(sshHandler.Routes()))
	}

	addr := *bind + ":" + fmt.Sprintf("%d", *port)
	srv := &http.Server{
		Addr:              addr,
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

	if *tlsCert != "" && *tlsKey != "" {
		srv.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12}
		log.Printf("api %s listening on https://%s (cors=%v)", apiVersion, addr, corsOrigins)
		if err := srv.ListenAndServeTLS(*tlsCert, *tlsKey); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server error: %v", err)
		}
	} else {
		log.Printf("api %s listening on http://%s (omp-bin=%q share-dir=%q no-encryption=%v)",
			apiVersion, addr, resolvedBin, effectiveShareDir, *noEncryption)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server error: %v", err)
		}
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
