package storage

import (
	"path/filepath"
	"testing"

	"github.com/lucaspdude/rocinante-harness/apps/api/internal/auth"
)

func TestOpenAndApplyMigrationsIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")
	db, err := Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	if err := ApplyMigrations(db); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if err := ApplyMigrations(db); err != nil {
		t.Fatalf("re-apply: %v", err)
	}
	row := db.QueryRow(`SELECT MAX(version) FROM schema_version`)
	var v int
	if err := row.Scan(&v); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if v != 2 {
		t.Errorf("version = %d, want 1", v)
	}
	rows, err := db.Query(`SELECT name FROM sqlite_master WHERE type='table' ORDER BY name`)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer rows.Close()
	seen := map[string]bool{}
	for rows.Next() {
		var n string
		_ = rows.Scan(&n)
		seen[n] = true
	}
	for _, want := range []string{"devices", "refresh_tokens", "audit", "pairing_codes", "schema_version"} {
		if !seen[want] {
			t.Errorf("missing table %q", want)
		}
	}
}

func TestRefreshStoreIssueAndLookup(t *testing.T) {
	dir := t.TempDir()
	db, err := Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	if err := ApplyMigrations(db); err != nil {
		t.Fatalf("apply: %v", err)
	}
	ds := auth.NewDeviceStore(db)
	dev := &auth.Device{
		ID:          "dev1",
		Name:        "test",
		PublicKeyID: "pk1",
		CreatedAt:   nowUTC(),
		LastSeenAt:  nowUTC(),
	}
	if err := ds.Create(dev); err != nil {
		t.Fatalf("create: %v", err)
	}
	rs := auth.NewRefreshStore(db)
	if err := rs.Issue("tok1", "fam1", "dev1", []byte("hash"), 24*3600*1e9); err != nil {
		t.Fatalf("issue: %v", err)
	}
	got, err := rs.Lookup("tok1")
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if got == nil || got.ID != "tok1" || got.FamilyID != "fam1" {
		t.Errorf("got = %+v", got)
	}
}

func TestRefreshStoreRevokeFamily(t *testing.T) {
	dir := t.TempDir()
	db, err := Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	if err := ApplyMigrations(db); err != nil {
		t.Fatalf("apply: %v", err)
	}
	ds := auth.NewDeviceStore(db)
	_ = ds.Create(&auth.Device{ID: "dev1", Name: "t", PublicKeyID: "p", CreatedAt: nowUTC(), LastSeenAt: nowUTC()})
	rs := auth.NewRefreshStore(db)
	_ = rs.Issue("a1", "fam1", "dev1", []byte("h1"), 24*3600*1e9)
	_ = rs.Issue("a2", "fam1", "dev1", []byte("h2"), 24*3600*1e9)
	if err := rs.RevokeFamily("fam1"); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	rows, err := db.Query(`SELECT COUNT(*) FROM refresh_tokens WHERE family_id = ? AND revoked_at IS NOT NULL`, "fam1")
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var c int
		_ = rows.Scan(&c)
		if c != 2 {
			t.Errorf("revoked = %d, want 2", c)
		}
	}
}

func TestDeviceStoreLifecycle(t *testing.T) {
	dir := t.TempDir()
	db, err := Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	if err := ApplyMigrations(db); err != nil {
		t.Fatalf("apply: %v", err)
	}
	ds := auth.NewDeviceStore(db)
	if err := ds.Create(&auth.Device{ID: "d1", Name: "n", PublicKeyID: "p", CreatedAt: nowUTC(), LastSeenAt: nowUTC()}); err != nil {
		t.Fatalf("create: %v", err)
	}
	got, err := ds.Get("d1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got == nil || got.Name != "n" {
		t.Errorf("got = %+v", got)
	}
	if err := ds.Revoke("d1"); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	got, _ = ds.Get("d1")
	if got == nil || got.RevokedAt == nil {
		t.Errorf("expected revoked_at set")
	}
}
