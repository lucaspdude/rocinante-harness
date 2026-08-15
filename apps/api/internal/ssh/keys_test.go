package ssh

import (
	"crypto/ed25519"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lucaspdude/rocinante-harness/apps/api/internal/storage"
)

func openDB(t *testing.T) (func(), string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")
	db, err := storage.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := storage.ApplyMigrations(db); err != nil {
		db.Close()
		t.Fatalf("migrate: %v", err)
	}
	return func() { db.Close() }, path
}

func TestKeyGenerateAndList(t *testing.T) {
	close, _ := openDB(t)
	defer close()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")
	db, err := storage.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	if err := storage.ApplyMigrations(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	store := NewKeyStore(db)
	key, priv, err := store.Generate("github", "github")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if key.ID == "" {
		t.Errorf("id empty")
	}
	if key.Fingerprint == "" {
		t.Errorf("fingerprint empty")
	}
	if !strings.HasPrefix(key.PublicKey, "ssh-ed25519 ") {
		t.Errorf("public key shape: %q", key.PublicKey)
	}
	if len(priv) != ed25519.PrivateKeySize {
		t.Errorf("private key length: %d", len(priv))
	}
	list, err := store.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("list size = %d, want 1", len(list))
	}
}

func TestKeyDelete(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")
	db, err := storage.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	if err := storage.ApplyMigrations(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	store := NewKeyStore(db)
	key, _, err := store.Generate("test", "")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if err := store.Delete(key.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	list, _ := store.List()
	if len(list) != 0 {
		t.Errorf("list size = %d, want 0", len(list))
	}
}

func TestServerCreateAndTest(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")
	db, err := storage.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	if err := storage.ApplyMigrations(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	keystore := NewKeyStore(db)
	srvStore := NewServerStore(db)
	key, _, err := keystore.Generate("k", "")
	if err != nil {
		t.Fatalf("keygen: %v", err)
	}
	srv, err := srvStore.Create("localhost", "127.0.0.1", 22, "lucas", key.ID)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if srv.Host != "127.0.0.1" || srv.Port != 22 {
		t.Errorf("server shape: %+v", srv)
	}
	got, err := srvStore.Get(srv.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Username != "lucas" {
		t.Errorf("username: %q", got.Username)
	}
}
