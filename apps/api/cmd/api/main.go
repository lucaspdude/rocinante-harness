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
	"runtime"
	"syscall"
	"time"

	"github.com/lucaspdude/rocinante-harness/apps/api/internal/api"
	"github.com/lucaspdude/rocinante-harness/apps/api/internal/api/middleware"
	"github.com/lucaspdude/rocinante-harness/apps/api/internal/auth"
	"github.com/lucaspdude/rocinante-harness/apps/api/internal/catalog"
	"github.com/lucaspdude/rocinante-harness/apps/api/internal/envconfig"
	"github.com/lucaspdude/rocinante-harness/apps/api/internal/files"
	"github.com/lucaspdude/rocinante-harness/apps/api/internal/keystore"
	"github.com/lucaspdude/rocinante-harness/apps/api/internal/omp"
	"github.com/lucaspdude/rocinante-harness/apps/api/internal/projects"
	sshpkg "github.com/lucaspdude/rocinante-harness/apps/api/internal/ssh"
	"github.com/lucaspdude/rocinante-harness/apps/api/internal/storage"
	"github.com/lucaspdude/rocinante-harness/apps/api/internal/sessions"
	"github.com/lucaspdude/rocinante-harness/apps/api/internal/clitools"

)

// apiVersion is set at build time via -ldflags "-X main.apiVersion=<tag>".
// The default "1.0.0" is used only for dev builds where ldflags did
// not inject a value; release.yml always passes the current tag.
var apiVersion = "1.0.0"

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
	shareDir := flag.String("share-dir", "", "base directory for key/db/logs (overrides ROCINANTE_SHARE_DIR)")
	noEncryption := flag.Bool("no-encryption", false, "store Ed25519 key in plaintext (dev only)")
	port := flag.Int("port", 30179, "HTTP port (overrides ROCINANTE_PORT)")
	tlsCert := flag.String("tls-cert", "", "TLS certificate path (PEM)")
	tlsKey := flag.String("tls-key", "", "TLS private key path (PEM)")
	bind := flag.String("bind", envOr("ROCINANTE_HOST", "127.0.0.1"), "host/IP to listen on (use 0.0.0.0 for LAN exposure)")
	showVersion := flag.Bool("version", false, "print version and exit")

	flag.Parse()

	if *showVersion {
		fmt.Printf("api %s\n", apiVersion)
		fmt.Printf("  go:    %s\n", runtime.Version())
		fmt.Printf("  os:    %s\n", runtime.GOOS)
		fmt.Printf("  arch:  %s\n", runtime.GOARCH)
		return
	}

	if len(flag.Args()) > 0 && flag.Args()[0] == "init" {
		dir := *shareDir
		if dir == "" {
			dir = envconfig.ShareDir()
		}
		if err := initShare(dir, *noEncryption, envconfig.PassphraseEnv()); err != nil {
			log.Fatalf("init: %v", err)
		}
		return
	}

	effectiveShareDir := *shareDir
	if effectiveShareDir == "" {
		effectiveShareDir = envconfig.ShareDir()
	}

	passphrase := ""
	if envName := envconfig.PassphraseEnv(); envName != "" {
		passphrase = os.Getenv(envName)
	}

	loader, resolvedBin := resolveOmp(envconfig.OmpBin())
	keystoreStore := keystore.New(effectiveShareDir)
	manager := omp.NewManagerWithEnv(resolvedBin, keystoreStore)
	// Phase-6 PR-1: write ~/.omp/agent/models.yml from the keystore on
	// every key add/remove and kill in-flight OMP sessions so the
	// next request picks up the new provider. Default target dir is
	// $OMP_AGENT_DIR or $HOME/.omp/agent — the same path the OMP
	// probe uses.
	modelsWriter := omp.NewModelsConfigWriter(os.Getenv("OMP_AGENT_DIR"))
	_ = modelsWriter.SyncIfConfigured(keystoreStore) // best-effort on boot
	idem := middleware.NewIdempotencyCache(2048)

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

	mux := http.NewServeMux()

	// PR-03: project registry + file-access allow-list. Migration runs
	// before the server starts so the first poll from the web returns
	// any pre-existing ompweb projects.
	projectReg := projects.NewRegistry(effectiveShareDir)
	if mr, err := projects.MigrateFromOmpweb(projectReg, effectiveShareDir); err != nil {
		log.Printf("warning: ompweb projects migration failed: %v", err)
	} else if mr.Added > 0 || mr.SkippedExisting > 0 {
		log.Printf("migrated %d ompweb project(s); skipped %d existing",
			mr.Added, mr.SkippedExisting)
	}
	fileAccess := files.NewFileAccess()
	for _, p := range projectReg.List() {
		fileAccess.QuietAllow(p.Path)
	}
	// PR-02: seed the picker allow-list with $HOME so the picker
	// can navigate the user's home dir before any project has
	// been registered. On systemd-less runs (rare) $HOME can be
	// empty; fall back to /root which is the harness default.
	home, _ := os.UserHomeDir()
	if home == "" {
		home = "/root"
	}
	fileAccess.QuietAllow(home)
	// PR-03: also allow the parent dir so the picker can climb
	// out of $HOME and browse / (and beyond). On LXC hosts where
	// $HOME=/root, filepath.Dir("/root") == "/", which is the
	// root the Finder-style picker wants to land on. The allow-
	// list still gates every other path, so this expansion only
	// opens the filesystem one level above the user's home.
	if parent := filepath.Dir(home); parent != home {
		fileAccess.QuietAllow(parent)
	}
	// PR-03: models.dev catalog handler. Refresh kicks off in the
	// background so the first GET /api/v1/models/catalog warms the
	// cache without blocking startup. Shares the same login-providers
	// cache as /api/v1/login so the cross-ref annotations match.
	loginProvidersCache := api.NewLoginProvidersCache(
		api.NewOMPLoginProviders(
			resolvedBin,
			&keystore.EnvProbe{Store: keystoreStore},
			api.NewStaticLoginProviders(&keystore.EnvProbe{Store: keystoreStore}),
		),
	)
	modelsCatalogHandler := api.NewModelsCatalogHandler(
		catalog.NewModelsDevCatalog(),
		loginProvidersCache,
		catalog.NewRatesCache(),
	)
	mux.Handle("/", middleware.TLSHandler(
		middleware.CORSHandler(middleware.CORSConfig{})(
		api.NewRouter(api.RouterDeps{
			MetaLoader:   loader,
			Manager:      manager,
			APIVersion:   apiVersion,
			Idempotency:  idem,
			AuthState:    authState,
			AuthMW:       authMW,
			ShareDir:     effectiveShareDir,
			ProviderKeys: keystoreStore,
			OMP:          manager,
			Models:       modelsWriter,
			LoginHandlers: &api.LoginHandlers{
				Providers:  loginProvidersCache,
				Jobs:       api.NewLoginJobs(),
				CmdFactory: func(ctx context.Context, name string, args []string) api.CmdIface {
					return api.OSExec(ctx, name, args...)
				},
			},
			ModelsCatalog: modelsCatalogHandler,
			Projects: &api.ProjectsHandlers{
				Registry:   projectReg,
				Sessions:   manager,
				Home:       home,
				FileAccess: fileAccess,
			},
			Clone: &api.CloneHandlers{
				Jobs:       projects.NewCloneJobs(),
				Registry:   projectReg,
				FileAccess: fileAccess,
			},
			Files: files.NewFilesHandler(fileAccess, home),
			Git:   files.NewGitHandler(fileAccess),
			CliTools: &api.CliToolsHandler{
				Manager: clitools.NewManager(),
			},
			SessionsStore: sessions.New(effectiveShareDir),
		}),
		),
	))
	if dbErr == nil && authMW != nil {
		sshHandler := &sshpkg.Handler{
			Keys:    sshpkg.NewKeyStore(db),
			Servers: sshpkg.NewServerStore(db),
			AuthMW:  authMW,
			Home:    home,
		}
		mux.Handle("/api/v1/ssh/", middleware.TLSHandler(sshHandler.Routes()))
	}
	// the only thing that talks to it) reaches us via the Next.js
	// rewrite on the same host. Public access is delegated to
	// whatever fronts the web server (Caddy, Cloudflare, a LAN IP
	// directly). This removes the CORS / bind / allowlist flags
	// that produced so much installer friction.
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
		log.Printf("api %s listening on https://%s", apiVersion, addr)
		if err := srv.ListenAndServeTLS(*tlsCert, *tlsKey); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server error: %v", err)
		}
	} else {
		log.Printf("api %s listening on http://%s (omp-bin=%q share-dir=%q)",
			apiVersion, addr, resolvedBin, effectiveShareDir)
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
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
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

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
