package omp

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// MaxSessionsConcurrent is the cap on simultaneous live sessions.
const MaxSessionsConcurrent = 32

// SessionRecord is the metadata returned by POST /api/v1/sessions.
type SessionRecord struct {
	ID              string    `json:"id"`
	OmpCwd          string    `json:"omp_cwd"`
	Cwd             string    `json:"cwd"`
	CreatedAt       time.Time `json:"created_at"`
	ProtocolVersion int       `json:"protocol_version"`
	State           string    `json:"state"`
	LastSeenAt      time.Time `json:"last_seen_at"`
}

// Manager owns the live set of sessions. Safe for concurrent use.
type Manager struct {
	mu       sync.RWMutex
	sessions map[string]*Session
	records  map[string]SessionRecord
	factory  SessionFactory
}

// SessionFactory is the seam used by tests.
type SessionFactory interface {
	NewSession(opts Options) (*Session, error)
}

type spawnFactory struct {
	bin string
}

func (f spawnFactory) NewSession(opts Options) (*Session, error) {
	return Spawn(opts.optsContext(), opts)
}

// NewManager returns an empty manager backed by default factory.
func NewManager(ompBin string) *Manager {
	return &Manager{
		sessions: make(map[string]*Session),
		records:  make(map[string]SessionRecord),
		factory:  spawnFactory{bin: ompBin},
	}
}

// NewManagerWithFactory is used by tests.
func NewManagerWithFactory(f SessionFactory) *Manager {
	return &Manager{
		sessions: make(map[string]*Session),
		records:  make(map[string]SessionRecord),
		factory:  f,
	}
}

// Create spawns a new omp session under the given omp_cwd.
func (m *Manager) Create(ompCwd string) (*SessionRecord, error) {
	if ompCwd == "" {
		return nil, errors.New("omp_cwd required")
	}
	m.mu.Lock()
	if len(m.sessions) >= MaxSessionsConcurrent {
		m.mu.Unlock()
		return nil, fmt.Errorf("max sessions concurrent: %d", MaxSessionsConcurrent)
	}
	m.mu.Unlock()

	sess, err := m.factory.NewSession(Options{OpBin: m.resolveBin(), Cwd: ompCwd})
	if err != nil {
		return nil, err
	}
	proto, _ := sess.Version()

	id := uuid.NewString()
	now := time.Now().UTC()
	rec := SessionRecord{
		ID:              id,
		OmpCwd:          ompCwd,
		Cwd:             ompCwd,
		CreatedAt:       now,
		ProtocolVersion: proto,
		State:           "running",
		LastSeenAt:      now,
	}

	m.mu.Lock()
	m.sessions[id] = sess
	m.records[id] = rec
	m.mu.Unlock()

	return &rec, nil
}

// Get returns the session or nil if missing.
func (m *Manager) Get(id string) *Session {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.sessions[id]
}

// Record returns the SessionRecord or nil if missing.
func (m *Manager) Record(id string) (SessionRecord, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	rec, ok := m.records[id]
	return rec, ok
}

// All returns a snapshot of every active session record.
func (m *Manager) All() []SessionRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]SessionRecord, 0, len(m.records))
	for _, rec := range m.records {
		out = append(out, rec)
	}
	return out
}

// Close removes the session and signals the child to exit.
func (m *Manager) Close(id string) error {
	m.mu.Lock()
	sess, ok := m.sessions[id]
	if ok {
		delete(m.sessions, id)
		delete(m.records, id)
	}
	m.mu.Unlock()
	if !ok {
		return errors.New("session_not_found")
	}
	return sess.Close()
}

// CloseAll walks every session and calls Close. Used on shutdown.
func (m *Manager) CloseAll() {
	m.mu.Lock()
	ids := make([]string, 0, len(m.sessions))
	for id := range m.sessions {
		ids = append(ids, id)
	}
	m.mu.Unlock()
	for _, id := range ids {
		_ = m.Close(id)
	}
}

// Count returns the live session count (used by tests).
func (m *Manager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.sessions)
}

func (m *Manager) resolveBin() string {
	if f, ok := m.factory.(spawnFactory); ok {
		return f.bin
	}
	return ""
}
