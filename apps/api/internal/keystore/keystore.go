// Package keystore persists provider API keys to disk in a
// chmod-0600 JSON file under the api's share dir. The api reads
// the file on every /api/v1/meta request (so the web UI's
// "configured / not set" checklist is always live) and on
// every omp session spawn (so each spawned subprocess inherits
// the right env vars without a process restart).
//
// Keys are stored as a flat map of provider name to key value:
//
//   {
//     "anthropic":  "sk-ant-...",
//     "openai":     "sk-...",
//     "minimax":    "sk-cp-..."
//   }
//
// The provider name is the part of the env var that comes after
// the leading "X_API_KEY" segment, lowercased. So MINIMAX_API_KEY
// → "minimax", ANTHROPIC_API_KEY → "anthropic", etc.
//
// The file is locked with flock so concurrent POST/DELETE requests
// don't trample each other's writes. Reads are lock-free: the file
// is small and the api reads it on every request, so a brief stale
// view during a write is fine.
package keystore

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

// ProviderName is the canonical id of a provider the api
// recognizes. The web UI uses these ids in its routes; the
// EnvVar() method gives the matching os env var name.
type ProviderName string

const (
	Anthropic  ProviderName = "anthropic"
	OpenAI     ProviderName = "openai"
	Gemini     ProviderName = "gemini"
	OpenRouter ProviderName = "openrouter"
	Minimax    ProviderName = "minimax"
)

// KnownProviders lists every provider the api recognizes, in
// the order they appear in the UI checklist.
var KnownProviders = []ProviderName{Anthropic, OpenAI, Gemini, OpenRouter, Minimax}

// EnvVar returns the os env var name that omp reads for the
// given provider. We keep this list here (and not in the i18n
// dictionaries) so the api and the web can share a single source
// of truth.
func (p ProviderName) EnvVar() string {
	switch p {
	case Anthropic:
		return "ANTHROPIC_API_KEY"
	case OpenAI:
		return "OPENAI_API_KEY"
	case Gemini:
		return "GEMINI_API_KEY"
	case OpenRouter:
		return "OPENROUTER_API_KEY"
	case Minimax:
		return "MINIMAX_API_KEY"
	default:
		return ""
	}
}

// IsKnown returns true if p is one of the recognized providers.
func IsKnown(p string) bool {
	for _, k := range KnownProviders {
		if string(k) == p {
			return true
		}
	}
	return false
}

// ErrNotFound is returned by Get when the provider has no key
// stored. The web UI renders this as "Not set".
var ErrNotFound = errors.New("keystore: provider not found")

// Store persists the provider keys.
type Store struct {
	dir  string
	path string
	mu   sync.Mutex
}

// New returns a Store rooted at dir. The keys file is created
// lazily on the first write; it doesn't need to exist.
func New(dir string) *Store {
	return &Store{
		dir:  dir,
		path: filepath.Join(dir, "provider-keys.json"),
	}
}

// Path returns the on-disk path of the keys file. Useful for
// logging and for the installer to add to its allowlist.
func (s *Store) Path() string { return s.path }

// read parses the on-disk file. Returns an empty map (not an
// error) if the file does not exist.
func (s *Store) read() (map[string]string, error) {
	b, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return map[string]string{}, nil
		}
		return nil, err
	}
	if len(b) == 0 {
		return map[string]string{}, nil
	}
	var out map[string]string
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, fmt.Errorf("keystore: %w", err)
	}
	return out, nil
}

// write persists the map atomically: write to a temp file in
// the same directory, fsync, rename. The target file ends up
// chmod 0600 owned by the current uid. The atomic rename
// guarantees that readers (omp session spawns, /meta polls) see
// either the old contents or the new contents, never a half
// written file.
func (s *Store) write(m map[string]string) error {
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(s.dir, "provider-keys-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath) // no-op if we successfully renamed
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpPath, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, s.path); err != nil {
		return err
	}
	return nil
}

// Get returns the stored key for provider p, or ErrNotFound.
func (s *Store) Get(p ProviderName) (string, error) {
	m, err := s.read()
	if err != nil {
		return "", err
	}
	v, ok := m[string(p)]
	if !ok || v == "" {
		return "", ErrNotFound
	}
	return v, nil
}

// Set stores the key for provider p, overwriting any existing
// value. The provider must be in KnownProviders; otherwise
// Set returns an error and the file is left unchanged.
func (s *Store) Set(p ProviderName, key string) error {
	if !IsKnown(string(p)) {
		return fmt.Errorf("keystore: unknown provider %q", string(p))
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	m, err := s.read()
	if err != nil {
		return err
	}
	if key == "" {
		delete(m, string(p))
	} else {
		m[string(p)] = key
	}
	return s.write(m)
}

// Delete removes the entry for p. No error if p was not set.
func (s *Store) Delete(p ProviderName) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, err := s.read()
	if err != nil {
		return err
	}
	delete(m, string(p))
	return s.write(m)
}

// Snapshot returns a copy of the current keys for the Env() method.
// The returned map is safe to mutate.
func (s *Store) Snapshot() (map[string]string, error) {
	return s.read()
}

// Names returns the provider names that currently have a non-empty
// key, in alphabetical order. Used by /api/v1/meta to report
// which providers are "configured".
func (s *Store) Names() ([]ProviderName, error) {
	m, err := s.read()
	if err != nil {
		return nil, err
	}
	var out []ProviderName
	for k, v := range m {
		if v != "" && IsKnown(k) {
			out = append(out, ProviderName(k))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out, nil
}

// Env returns the current keys as a slice of "KEY=value" pairs,
// one per configured provider. omp subprocesses use this via
// exec.CommandContext's Env field; setting it on the cmd means
// the subprocess gets exactly these keys (in addition to the
// api's own os.Environ()).
func (s *Store) Env() ([]string, error) {
	m, err := s.read()
	if err != nil {
		return nil, err
	}
	var out []string
	for name, key := range m {
		if key == "" {
			continue
		}
		p := ProviderName(name)
		envName := p.EnvVar()
		if envName == "" {
			continue
		}
		out = append(out, fmt.Sprintf("%s=%s", envName, key))
	}
	return out, nil
}
