package ssh

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ConfigBlock is one Host block appended (or replaced) in
// ~/.ssh/config. Aliases lists every `Host` alias this block
// answers to — the first one is the identity used for replace-
// detection. HostName is what SSH dials; User is the SSH user
// (always "git" for the Git providers). IdentityFile is the
// absolute path to the private key.
type ConfigBlock struct {
	Aliases      []string
	HostName     string
	User         string
	IdentityFile string
	// Optional fields with sensible defaults. Pass empty to use the
	// default — these exist so we can override per provider without
	// growing the struct every release.
	IdentitiesOnly       *bool
	StrictHostKeyChecking string
	Port                 string
}

// Render produces the SSH config text for this block, with a
// trailing blank line so the next Host section is unambiguous.
func (b ConfigBlock) Render() string {
	var sb strings.Builder
	// SSH parses "Host" as a pattern list; if Aliases has more
	// than one entry we keep them on a single line, space-separated,
	// matching OpenSSH's behaviour for multi-alias blocks.
	aliases := strings.Join(b.Aliases, " ")
	sb.WriteString("Host " + aliases + "\n")
	if b.HostName != "" {
		sb.WriteString("  HostName " + b.HostName + "\n")
	}
	if b.User != "" {
		sb.WriteString("  User " + b.User + "\n")
	}
	if b.Port != "" {
		sb.WriteString("  Port " + b.Port + "\n")
	}
	if b.IdentityFile != "" {
		sb.WriteString("  IdentityFile " + b.IdentityFile + "\n")
	}
	if b.IdentitiesOnly != nil {
		if *b.IdentitiesOnly {
			sb.WriteString("  IdentitiesOnly yes\n")
		} else {
			sb.WriteString("  IdentitiesOnly no\n")
		}
	}
	if b.StrictHostKeyChecking != "" {
		sb.WriteString("  StrictHostKeyChecking " + b.StrictHostKeyChecking + "\n")
	}
	sb.WriteString("\n")
	return sb.String()
}

// FirstAlias returns the identity alias used for replace-detection.
// Empty if the block has no aliases.
func (b ConfigBlock) FirstAlias() string {
	if len(b.Aliases) == 0 {
		return ""
	}
	return b.Aliases[0]
}

// configPath returns the absolute path to ~/.ssh/config for the
// current user. It does not create the directory; EnsureSshDir
// must be called first.
func configPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("home dir: %w", err)
	}
	if home == "" {
		return "", errors.New("home dir is empty")
	}
	return filepath.Join(home, ".ssh", "config"), nil
}

// readConfig returns the current contents of ~/.ssh/config ("" if
// the file does not exist yet — that's a fresh-install case).
func readConfig() (string, error) {
	p, err := configPath()
	if err != nil {
		return "", err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", fmt.Errorf("read %s: %w", p, err)
	}
	return string(b), nil
}

// writeConfigAtomic writes the new config text via a temp file +
// rename, so partial reads never observe a truncated file.
func writeConfigAtomic(content string) error {
	p, err := configPath()
	if err != nil {
		return err
	}
	dir := filepath.Dir(p)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, ".config-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpName := tmp.Name()
	// Ensure the temp file lives under ~/.ssh — AssertInSshDir
	// rejects anything outside, which is what we want.
	if err := AssertInSshDir(tmpName); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if _, err := tmp.Write([]byte(content)); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("write temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("close temp: %w", err)
	}
	// Mode 0o600 matches ssh's default for the config file when it
	// only contains user blocks. If the user later adds system-wide
	// config (Match all), they can chmod themselves.
	if err := os.Chmod(tmpName, 0o600); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("chmod temp: %w", err)
	}
	if err := os.Rename(tmpName, p); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}

// AppendConfigBlock inserts (or replaces) the given block in
// ~/.ssh/config. If a block whose first alias matches
// block.FirstAlias() already exists, it is replaced in place;
// otherwise the new block is appended at the end of the file.
// The function is safe to call concurrently for distinct first
// aliases — last writer wins for the same alias, but the result
// is always a well-formed config file.
func AppendConfigBlock(block ConfigBlock) error {
	if block.FirstAlias() == "" {
		return errors.New("config block requires at least one alias")
	}
	if block.IdentityFile == "" {
		return errors.New("config block requires IdentityFile")
	}
	current, err := readConfig()
	if err != nil {
		return err
	}
	replaced := false
	updated := replaceBlock(current, block, &replaced)
	if !replaced {
		updated = strings.TrimRight(updated, "\n") + "\n\n" + block.Render()
	}
	return writeConfigAtomic(updated)
}

// replaceBlock walks the config text block-by-block (each block
// being a `Host ...` stanza through the next blank line or EOF)
// and rewrites any stanza whose first `Host` token matches the
// replacement's FirstAlias(). The boolean `replaced` is set to
// true iff a rewrite actually happened; the caller uses it to
// decide between in-place edit vs. append.
func replaceBlock(text string, block ConfigBlock, replaced *bool) string {
	if text == "" {
		return text
	}
	target := block.FirstAlias()
	lines := strings.Split(text, "\n")
	var sb strings.Builder
	// We accumulate the current stanza until we either hit a blank
	// line or EOF, then decide whether to keep or replace it.
	i := 0
	first := true
	for i < len(lines) {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		if first && trimmed == "" {
			// Leading blank lines — preserve verbatim.
			sb.WriteString(line)
			sb.WriteString("\n")
			i++
			continue
		}
		first = false
		if !strings.HasPrefix(trimmed, "Host ") && !strings.HasPrefix(trimmed, "Match ") {
			// Not the start of a Host stanza; emit as-is.
			sb.WriteString(line)
			sb.WriteString("\n")
			i++
			continue
		}
		// We're at the start of a stanza — slurp it.
		var stanza []string
		for i < len(lines) {
			sl := lines[i]
			if strings.TrimSpace(sl) == "" {
				// blank line terminates the stanza but we let the
				// outer loop emit it so file shape stays intact.
				break
			}
			stanza = append(stanza, sl)
			i++
		}
		stanzaText := strings.Join(stanza, "\n")
		firstAlias := firstAliasOf(stanza)
		if firstAlias == target {
			// Replace — drop original, write new block.
			sb.WriteString(block.Render())
			*replaced = true
		} else {
			sb.WriteString(stanzaText)
			sb.WriteString("\n")
		}
		// Consume one trailing blank line if present so the outer
		// loop doesn't double-emit it.
		if i < len(lines) && strings.TrimSpace(lines[i]) == "" {
			sb.WriteString("\n")
			i++
		}
	}
	return sb.String()
}

// firstAliasOf returns the first token after `Host` in a stanza
// header, or "" if the first line is not a `Host` directive.
func firstAliasOf(stanza []string) string {
	if len(stanza) == 0 {
		return ""
	}
	header := strings.TrimSpace(stanza[0])
	if !strings.HasPrefix(header, "Host ") {
		return ""
	}
	rest := strings.TrimSpace(strings.TrimPrefix(header, "Host "))
	fields := strings.Fields(rest)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}
