// Package projects owns the project registry (Phase 4 of
// rocinante-harness). Projects are path → metadata with CRUD
// semantics. PR-03 (docs/mvp/phase-1-functionality/05-pr-specs/
// PR-03-projects-registry.md) wires the api endpoints; PR-4 adds
// the clone SSE; PR-5+ the web UI.
package projects

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// Project is the canonical entry in <share-dir>/projects.json.
type Project struct {
	Path        string            `json:"path"`        // absolute path; canonical realpath
	Name        string            `json:"name"`        // user-facing label (defaults to basename)
	Description string            `json:"description,omitempty"`
	AddedAt     time.Time         `json:"added_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	Hidden      bool              `json:"hidden,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// Registry persists Projects to a single JSON file under dir.
// Lock is sync.RWMutex — reads via RLock, writes via Lock.
type Registry struct {
	mu   sync.RWMutex
	dir  string
	path string
	data map[string]*Project
}

// NewRegistry opens (or initializes) the registry at <dir>/projects.json.
func NewRegistry(dir string) *Registry {
	r := &Registry{
		dir:  dir,
		path: filepath.Join(dir, "projects.json"),
		data: make(map[string]*Project),
	}
	_ = r.load()
	return r
}

// load reads from disk; missing file is not an error.
func (r *Registry) load() error {
	b, err := os.ReadFile(r.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("read: %w", err)
	}
	if len(b) == 0 {
		return nil
	}
	var raw map[string]*Project
	if err := json.Unmarshal(b, &raw); err != nil {
		return fmt.Errorf("decode: %w", err)
	}
	if raw == nil {
		raw = make(map[string]*Project)
	}
	r.data = raw
	return nil
}

// save writes the registry atomically: temp + rename.
func (r *Registry) save() error {
	if err := os.MkdirAll(r.dir, 0o700); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	b, err := json.MarshalIndent(r.data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	tmp, err := os.CreateTemp(r.dir, "projects-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath) // no-op if rename succeeded
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
	return os.Rename(tmpPath, r.path)
}

// List returns a snapshot of the registry, sorted by Path.
func (r *Registry) List() []Project {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Project, 0, len(r.data))
	for _, p := range r.data {
		out = append(out, *p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

// Get returns the project at path, or nil if absent. The boolean
// distinguishes hidden vs missing.
func (r *Registry) Get(path string) (*Project, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.data[path]
	if !ok {
		return nil, false
	}
	cp := *p
	return &cp, true
}

// ErrAlreadyRegistered is returned by Upsert when called with
// allowUpdate=false and the path is already in the registry.
var ErrAlreadyRegistered = errors.New("already_registered")

// Upsert registers or updates a project.
func (r *Registry) Upsert(path, name, description string, allowUpdate bool) (*Project, error) {
	if path == "" {
		return nil, errors.New("path required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if existing, ok := r.data[path]; ok {
		if !allowUpdate {
			return existing, ErrAlreadyRegistered
		}
		existing.UpdatedAt = time.Now().UTC()
		if name != "" {
			existing.Name = name
		}
		if description != "" {
			existing.Description = description
		}
		cp := *existing
		if err := r.save(); err != nil {
			return nil, err
		}
		return &cp, nil
	}
	now := time.Now().UTC()
	p := &Project{
		Path:        path,
		Name:        name,
		Description: description,
		AddedAt:     now,
		UpdatedAt:   now,
		Hidden:      false,
	}
	if name == "" {
		p.Name = filepath.Base(path)
	}
	r.data[path] = p
	cp := *p
	if err := r.save(); err != nil {
		return nil, err
	}
	return &cp, nil
}

// Patch updates project fields without registering a new one.
func (r *Registry) Patch(path string, name *string, description *string) (*Project, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.data[path]
	if !ok {
		return nil, errors.New("not_found")
	}
	if name != nil {
		p.Name = *name
	}
	if description != nil {
		p.Description = *description
	}
	p.UpdatedAt = time.Now().UTC()
	cp := *p
	if err := r.save(); err != nil {
		return nil, err
	}
	return &cp, nil
}

// Hide marks a project hidden (true); restores (false).
func (r *Registry) Hide(path string, hidden bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.data[path]
	if !ok {
		return errors.New("not_found")
	}
	p.Hidden = hidden
	p.UpdatedAt = time.Now().UTC()
	return r.save()
}

// IsRegistered reports whether the given path is in the registry,
// regardless of hidden.
func (r *Registry) IsRegistered(path string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.data[path]
	return ok
}

// MigrateFromOmpweb imports entries from ompweb's
// ~/.omp/agent/projects.json file. The upstream schema is tolerant
// (some entries are timestamps/ints); we skip entries that don't
// decode as project objects.
func (r *Registry) MigrateFromOmpweb(srcPath string) (added int, skipped int, err error) {
	b, err := os.ReadFile(srcPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, 0, nil
		}
		return 0, 0, err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		return 0, 0, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for path, payload := range raw {
		if _, ok := r.data[path]; ok {
			skipped++
			continue
		}
		var op ompwebProject
		if err := json.Unmarshal(payload, &op); err != nil {
			// Skip entries that aren't valid project objects
			// (timestamps, ints, etc).
			skipped++
			continue
		}
		now := time.Now().UTC()
		name := op.Name
		if name == "" {
			name = filepath.Base(path)
		}
		r.data[path] = &Project{
			Path:        path,
			Name:        name,
			Description: op.Description,
			AddedAt:     now,
			UpdatedAt:   now,
			Hidden:      false,
			Metadata:    op.Metadata,
		}
		added++
	}
	if err := r.save(); err != nil {
		return added, skipped, err
	}
	return added, skipped, nil
}

// ompwebProject is the upstream schema from ompweb's
// projects.json (see docs/finished/2026-08-12-...).
type ompwebProject struct {
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}
