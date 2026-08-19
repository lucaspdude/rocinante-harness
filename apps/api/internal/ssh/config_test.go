package ssh

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// helper that points $HOME at a temp dir and ensures ~/.ssh.
func freshHome(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	if _, err := EnsureSshDir(); err != nil {
		t.Fatalf("EnsureSshDir: %v", err)
	}
	return tmp
}

// TestAppendConfigBlockFresh ensures a brand-new ~/.ssh/config gets
// the block written verbatim with a trailing blank line.
func TestAppendConfigBlockFresh(t *testing.T) {
	freshHome(t)
	err := AppendConfigBlock(ConfigBlock{
		Aliases:      []string{"github.com"},
		HostName:     "github.com",
		User:         "git",
		IdentityFile: "~/.ssh/id_ed25519_github",
	})
	if err != nil {
		t.Fatalf("AppendConfigBlock: %v", err)
	}
	got, err := readConfig()
	if err != nil {
		t.Fatalf("readConfig: %v", err)
	}
	if !strings.Contains(got, "Host github.com") {
		t.Errorf("missing Host directive:\n%s", got)
	}
	if !strings.Contains(got, "IdentityFile ~/.ssh/id_ed25519_github") {
		t.Errorf("missing IdentityFile:\n%s", got)
	}
	if !strings.Contains(got, "User git") {
		t.Errorf("missing User:\n%s", got)
	}
	if !strings.HasSuffix(got, "\n\n") {
		t.Errorf("expected trailing blank line, got %q", got)
	}
}

// TestAppendConfigBlockReplacesExisting verifies that calling
// AppendConfigBlock twice with the same first alias produces one
// Host stanza (the second call replaces, not appends).
func TestAppendConfigBlockReplacesExisting(t *testing.T) {
	tmp := freshHome(t)
	cfgPath := filepath.Join(tmp, ".ssh", "config")
	if err := os.WriteFile(cfgPath, []byte("# prelude\n\nHost *\n  ServerAliveInterval 30\n\n"), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	first := ConfigBlock{
		Aliases:      []string{"github.com"},
		HostName:     "github.com",
		User:         "git",
		IdentityFile: "~/.ssh/id_ed25519_old",
	}
	if err := AppendConfigBlock(first); err != nil {
		t.Fatalf("first append: %v", err)
	}
	second := ConfigBlock{
		Aliases:      []string{"github.com"},
		HostName:     "github.com",
		User:         "git",
		IdentityFile: "~/.ssh/id_ed25519_new",
	}
	if err := AppendConfigBlock(second); err != nil {
		t.Fatalf("second append: %v", err)
	}
	got, err := readConfig()
	if err != nil {
		t.Fatalf("readConfig: %v", err)
	}
	if strings.Contains(got, "id_ed25519_old") {
		t.Errorf("old IdentityFile still present after replace:\n%s", got)
	}
	if !strings.Contains(got, "id_ed25519_new") {
		t.Errorf("new IdentityFile missing after replace:\n%s", got)
	}
	if strings.Count(got, "Host github.com") != 1 {
		t.Errorf("expected exactly one Host github.com stanza, got %d:\n%s", strings.Count(got, "Host github.com"), got)
	}
	// Prelude and the global Host * block must be preserved.
	if !strings.Contains(got, "# prelude") {
		t.Errorf("prelude lost:\n%s", got)
	}
	if !strings.Contains(got, "Host *") {
		t.Errorf("global Host * block lost:\n%s", got)
	}
}

// TestAppendConfigBlockMultiAlias verifies the Azure DevOps shape:
// two aliases sharing one IdentityFile in the same stanza.
func TestAppendConfigBlockMultiAlias(t *testing.T) {
	freshHome(t)
	err := AppendConfigBlock(ConfigBlock{
		Aliases:      []string{"dev.azure.com", "vs-ssh.visualstudio.com"},
		HostName:     "dev.azure.com",
		User:         "git",
		IdentityFile: "~/.ssh/id_ed25519_azure",
	})
	if err != nil {
		t.Fatalf("AppendConfigBlock: %v", err)
	}
	got, err := readConfig()
	if err != nil {
		t.Fatalf("readConfig: %v", err)
	}
	// Both aliases on a single Host line.
	if !strings.Contains(got, "Host dev.azure.com vs-ssh.visualstudio.com") {
		t.Errorf("multi-alias Host line missing:\n%s", got)
	}
	// Single IdentityFile under the stanza.
	if strings.Count(got, "IdentityFile") != 1 {
		t.Errorf("expected 1 IdentityFile, got %d:\n%s", strings.Count(got, "IdentityFile"), got)
	}
}

// TestAppendConfigBlockValidates verifies that blocks without an
// alias or without IdentityFile are rejected before any FS write.
func TestAppendConfigBlockValidates(t *testing.T) {
	freshHome(t)
	cases := []ConfigBlock{
		{HostName: "x", IdentityFile: "~/.ssh/k"},                                  // no aliases
		{Aliases: []string{"a"}, HostName: "x"},                                     // no identity
	}
	for i, b := range cases {
		if err := AppendConfigBlock(b); err == nil {
			t.Errorf("case %d: expected validation error", i)
		}
	}
	// And confirm the file is still empty.
	got, err := readConfig()
	if err != nil {
		t.Fatalf("readConfig: %v", err)
	}
	if got != "" {
		t.Errorf("file unexpectedly non-empty: %q", got)
	}
}

// TestReplaceBlockUnit directly exercises the in-memory parser
// with a hand-crafted config to check edge cases (multiple Host
// stanzas, blank-line handling).
func TestReplaceBlockUnit(t *testing.T) {
	input := `# top comment

Host github.com
  HostName github.com
  User git
  IdentityFile ~/.ssh/old

Host dev.azure.com vs-ssh.visualstudio.com
  HostName dev.azure.com
  User git
  IdentityFile ~/.ssh/azure-old

Host *
  ServerAliveInterval 30
`
	replaced := false
	updated := replaceBlock(input, ConfigBlock{
		Aliases:      []string{"github.com"},
		HostName:     "github.com",
		User:         "git",
		IdentityFile: "~/.ssh/new",
	}, &replaced)
	if !replaced {
		t.Errorf("expected replaced=true")
	}
	if strings.Contains(updated, "id_ed25519_old") {
		t.Errorf("old IdentityFile still present after replace:\n%s", updated)
	}
	if !strings.Contains(updated, "~/.ssh/new") {
		t.Errorf("'new' missing:\n%s", updated)
	}
	if !strings.Contains(updated, "Host dev.azure.com vs-ssh.visualstudio.com") {
		t.Errorf("Azure stanza lost:\n%s", updated)
	}
	if !strings.Contains(updated, "Host *") {
		t.Errorf("global stanza lost:\n%s", updated)
	}
	if !strings.Contains(updated, "# top comment") {
		t.Errorf("top comment lost:\n%s", updated)
	}
}
