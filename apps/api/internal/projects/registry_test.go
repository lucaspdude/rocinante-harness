package projects

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestRegistryUpsertGet(t *testing.T) {
	dir := t.TempDir()
	r := NewRegistry(dir)
	p, err := r.Upsert("/tmp/foo", "My Foo", "first test", false)
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if p.Path != "/tmp/foo" || p.Name != "My Foo" {
		t.Errorf("got %+v", p)
	}
	if _, err := r.Upsert("/tmp/foo", "x", "", false); err == nil {
		t.Error("expected already_registered error")
	}
	if _, err := r.Upsert("/tmp/foo", "Foo renamed", "", true); err != nil {
		t.Errorf("allowUpdate: %v", err)
	}
	got, ok := r.Get("/tmp/foo")
	if !ok || got.Name != "Foo renamed" {
		t.Errorf("after rename: got %+v", got)
	}
	if got.Description != "first test" {
		t.Errorf("description dropped: %q", got.Description)
	}
}

func TestRegistryPatch(t *testing.T) {
	dir := t.TempDir()
	r := NewRegistry(dir)
	_, _ = r.Upsert("/tmp/bar", "Bar", "", false)
	newname := "Baz"
	newdesc := "new description"
	got, err := r.Patch("/tmp/bar", &newname, &newdesc)
	if err != nil {
		t.Fatalf("patch: %v", err)
	}
	if got.Name != "Baz" || got.Description != "new description" {
		t.Errorf("patch: %+v", got)
	}
	if _, err := r.Patch("/no/such", nil, nil); err == nil {
		t.Error("expected not_found")
	}
}

func TestRegistryHide(t *testing.T) {
	dir := t.TempDir()
	r := NewRegistry(dir)
	_, _ = r.Upsert("/tmp/qux", "Qux", "", false)
	if err := r.Hide("/tmp/qux", true); err != nil {
		t.Fatalf("hide: %v", err)
	}
	if !r.IsRegistered("/tmp/qux") {
		t.Error("hide should keep entry in registry")
	}
	if err := r.Hide("/tmp/qux", false); err != nil {
		t.Fatalf("unhide: %v", err)
	}
	if err := r.Hide("/no/such/path", true); err == nil {
		t.Error("expected not_found")
	}
}

func TestRegistryPersistsAcross(t *testing.T) {
	dir := t.TempDir()
	r1 := NewRegistry(dir)
	_, _ = r1.Upsert("/tmp/persist", "Persist", "", false)
	r2 := NewRegistry(dir)
	got, ok := r2.Get("/tmp/persist")
	if !ok {
		t.Fatal("entry not loaded on reopen")
	}
	if got.Name != "Persist" {
		t.Errorf("got %+v", got)
	}
}

func TestRegistryConcurrentUpsert(t *testing.T) {
	dir := t.TempDir()
	r := NewRegistry(dir)
	var wg sync.WaitGroup
	for i := range 50 {
		idx := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = r.Upsert(
				filepath.Join("/tmp/concurrent", "p"+string(rune('a'+idx%26))),
				"x", "", true)
		}()
	}
	wg.Wait()
	all := r.List()
	if len(all) == 0 {
		t.Errorf("expected entries after concurrent upserts")
	}
}

func TestMigrateFromOmpwebNoSource(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	r := NewRegistry(dir)
	res, err := MigrateFromOmpweb(r, dir)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if !res.Skipped {
		t.Errorf("expected skipped=true; got %+v", res)
	}
	if _, err := os.Stat(MarkerPath(dir)); err != nil {
		t.Errorf("marker missing: %v", err)
	}
}

func TestMigrateFromOmpwebIdempotent(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	r := NewRegistry(dir)
	if _, err := MigrateFromOmpweb(r, dir); err != nil {
		t.Fatal(err)
	}
	res, err := MigrateFromOmpweb(r, dir)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Skipped {
		t.Errorf("second call should be skipped; got %+v", res)
	}
}

func TestMigrateFromOmpwebWithFakeSrc(t *testing.T) {
	dir := t.TempDir()
	fakeOmpDir := t.TempDir()
	t.Setenv("HOME", fakeOmpDir)
	agentDir := filepath.Join(fakeOmpDir, ".omp", "agent")
	if err := os.MkdirAll(agentDir, 0o700); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(agentDir, "projects.json")
	body := `{
		"/tmp/from-ompweb": {"name": "Ompweb project", "metadata": {"x": "y"}},
		"/tmp/conflict":    {"name": "Ompweb conflict"}
	}`
	if err := os.WriteFile(src, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	r := NewRegistry(dir)
	_, _ = r.Upsert("/tmp/conflict", "Pre-existing", "", false)

	res, err := MigrateFromOmpweb(r, dir)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if res.Added != 1 {
		t.Errorf("Added = %d, want 1", res.Added)
	}
	if res.SkippedExisting != 1 {
		t.Errorf("SkippedExisting = %d, want 1", res.SkippedExisting)
	}
	imp, ok := r.Get("/tmp/from-ompweb")
	if !ok {
		t.Fatal("imported entry missing")
	}
	if imp.Name != "Ompweb project" {
		t.Errorf("name = %q, want Ompweb project", imp.Name)
	}
	if imp.Metadata["x"] != "y" {
		t.Errorf("metadata lost: %+v", imp.Metadata)
	}
	ex, _ := r.Get("/tmp/conflict")
	if ex.Name != "Pre-existing" {
		t.Errorf("existing overwritten: name=%q", ex.Name)
	}
	if _, err := os.Stat(MarkerPath(dir)); err != nil {
		t.Errorf("marker missing: %v", err)
	}
	if !strings.HasSuffix(MarkerPath(dir), ".migration-completed") {
		t.Errorf("MarkerPath = %q", MarkerPath(dir))
	}
}
