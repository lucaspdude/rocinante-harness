package keystore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)

	// Empty store: Get returns ErrNotFound.
	if _, err := s.Get(Anthropic); err != ErrNotFound {
		t.Fatalf("empty Get: want ErrNotFound, got %v", err)
	}

	// Set, then Get.
	if err := s.Set(Anthropic, "sk-ant-test"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, err := s.Get(Anthropic)
	if err != nil {
		t.Fatalf("Get after Set: %v", err)
	}
	if got != "sk-ant-test" {
		t.Fatalf("Get: want %q, got %q", "sk-ant-test", got)
	}

	// Names includes Anthropic.
	names, err := s.Names()
	if err != nil {
		t.Fatalf("Names: %v", err)
	}
	if len(names) != 1 || names[0] != Anthropic {
		t.Fatalf("Names: want [anthropic], got %v", names)
	}

	// Env returns the matching os env var.
	env, err := s.Env()
	if err != nil {
		t.Fatalf("Env: %v", err)
	}
	want := "ANTHROPIC_API_KEY=sk-ant-test"
	if len(env) != 1 || env[0] != want {
		t.Fatalf("Env: want [%q], got %v", want, env)
	}

	// File is 0600.
	info, err := os.Stat(s.Path())
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Fatalf("file mode: want 0600, got %o", mode)
	}

	// Delete clears the entry.
	if err := s.Delete(Anthropic); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := s.Get(Anthropic); err != ErrNotFound {
		t.Fatalf("after Delete: want ErrNotFound, got %v", err)
	}
}

func TestSetUnknown(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)
	if err := s.Set(ProviderName("garbage"), "x"); err == nil {
		t.Fatal("Set unknown: want error, got nil")
	}
	// File should not be created.
	if _, err := os.Stat(s.Path()); !os.IsNotExist(err) {
		t.Fatalf("file should not exist; stat err = %v", err)
	}
}

func TestEnvVar(t *testing.T) {
	cases := map[ProviderName]string{
		Anthropic:  "ANTHROPIC_API_KEY",
		OpenAI:     "OPENAI_API_KEY",
		Gemini:     "GEMINI_API_KEY",
		OpenRouter: "OPENROUTER_API_KEY",
		Minimax:    "MINIMAX_API_KEY",
	}
	for p, want := range cases {
		if got := p.EnvVar(); got != want {
			t.Fatalf("%s.EnvVar: want %q, got %q", p, want, got)
		}
	}
	if got := ProviderName("garbage").EnvVar(); got != "" {
		t.Fatalf("unknown EnvVar: want empty, got %q", got)
	}
}

func TestIsKnown(t *testing.T) {
	for _, p := range KnownProviders {
		if !IsKnown(string(p)) {
			t.Fatalf("IsKnown(%q) want true", string(p))
		}
	}
	if IsKnown("garbage") {
		t.Fatal("IsKnown(garbage) want false")
	}
}

func TestEmptyKeyDeletes(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)
	if err := s.Set(OpenAI, "sk-x"); err != nil {
		t.Fatal(err)
	}
	// Setting an empty key is the same as Delete.
	if err := s.Set(OpenAI, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(OpenAI); err != ErrNotFound {
		t.Fatalf("after Set empty: want ErrNotFound, got %v", err)
	}
}

func TestAtomicWriteNoHalfFiles(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)
	if err := s.Set(Minimax, "sk-cp-x"); err != nil {
		t.Fatal(err)
	}
	// No leftover *.tmp files in the dir.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") {
			t.Fatalf("leftover tmp file: %s", e.Name())
		}
	}
	// The actual file exists and parses.
	b, err := os.ReadFile(filepath.Join(dir, "provider-keys.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "MINIMAX_API_KEY") &&
		!strings.Contains(string(b), "minimax") {
		t.Fatalf("file does not contain the minimax entry: %s", b)
	}
}
