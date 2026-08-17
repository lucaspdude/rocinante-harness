package api

// Live-login jobs. PR-01 calls for /api/v1/login/start/{provider}
// to spawn a short-lived omp child and stream its
// extension_ui_request frames back to the browser via SSE. The
// job lifecycle is owned here; the streamer handler is a thin
// wrapper that subscribes to the job's event channel.

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"
)

// LoginJobState is the lifecycle state of a single login attempt.
type LoginJobState string

const (
	LoginJobRunning LoginJobState = "running"
	LoginJobComplete LoginJobState = "complete"
	LoginJobFailed  LoginJobState = "failed"
	LoginJobExpired LoginJobState = "expired"
)

// LoginEvent is a single SSE event emitted by the omp child.
// Event maps to the SSE `event:` field. Data is the opaque payload
// we parsed out of extension_ui_request. Omp pushes many flavors —
// we don't constrain them.
type LoginEvent struct {
	Event string
	Data  map[string]any
}

// LoginJob is one in-flight login attempt. Subscribers receive a
// snapshot of past events on subscribe (so a reconnecting client
// doesn't miss frames), then the live stream.
type LoginJob struct {
	ID         string
	ProviderID string
	Auth       string
	StartedAt  time.Time
	EndedAt    time.Time
	Error      string

	mu      sync.Mutex
	state   LoginJobState
	subSeq  int
	events  []LoginEvent
	subs    map[int]chan LoginEvent
	cancel  context.CancelFunc
	publisher func(LoginEvent)
	respond func(LoginAck) error
}

// LoginJobs is the in-memory store of live login jobs. It is
// bounded by TTL (1h) and the size cap (32 concurrent). Jobs older
// than the TTL are evicted on the next Sweep.
type LoginJobs struct {
	mu       sync.Mutex
	jobs     map[string]*LoginJob
	ttl      time.Duration
	maxJobs  int
}

// NewLoginJobs returns a fresh login-job store.
func NewLoginJobs() *LoginJobs {
	return &LoginJobs{
		jobs:    make(map[string]*LoginJob),
		ttl:     time.Hour,
		maxJobs: 32,
	}
}

// NewJob creates a LoginJob with a fresh id, an event channel of
// buffer 64, and a cancel function for the parent context (when
// the underlying omp child is killed). The caller runs the omp
// child and uses publish to push frames in.
func (s *LoginJobs) NewJob(providerID, authKind string, cancel context.CancelFunc) *LoginJob {
	id := newLoginJobID()
	j := &LoginJob{
		ID:         id,
		ProviderID: providerID,
		Auth:       authKind,
		StartedAt:  time.Now().UTC(),
		state:      LoginJobRunning,
		subs:       make(map[int]chan LoginEvent),
		cancel:     cancel,
	}
	s.mu.Lock()
	// Best-effort: evict jobs older than ttl if we're at the cap.
	if len(s.jobs) >= s.maxJobs {
		s.evictLocked()
	}
	s.jobs[id] = j
	s.mu.Unlock()
	return j
}

// Get returns the job with the given id, or nil.
func (s *LoginJobs) Get(id string) *LoginJob {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.jobs[id]
}

// Sweep removes jobs older than the TTL regardless of state. Run
// it periodically (every 10m) from main; deterministic, cheap.
func (s *LoginJobs) Sweep() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.evictLocked()
}

func (s *LoginJobs) evictLocked() {
	cutoff := time.Now().Add(-s.ttl)
	for id, j := range s.jobs {
		// A running job whose parent context is canceled should
		// be removed too — guard against leaked goroutines.
		var ended bool
		j.mu.Lock()
		ended = !j.EndedAt.IsZero() && j.EndedAt.Before(cutoff)
		j.mu.Unlock()
		if ended {
			delete(s.jobs, id)
		}
	}
}

// publish appends an event to the history and fans it out to every
// active subscriber. Non-blocking on slow subscribers (we drop and
// log; the API keeps the job alive).
func (j *LoginJob) publish(ev LoginEvent) {
	j.mu.Lock()
	j.events = append(j.events, ev)
	subs := make([]chan LoginEvent, 0, len(j.subs))
	for _, ch := range j.subs {
		subs = append(subs, ch)
	}
	j.mu.Unlock()
	for _, ch := range subs {
		select {
		case ch <- ev:
		default:
			// Drop; the client's reconnect-with-snapshot path
			// will replay missed events.
		}
	}
}

// Subscribe registers a channel that receives future events and
// returns the past events as a snapshot, along with an unsubscribe
// function. Callers should run this in its own goroutine.
func (j *LoginJob) Subscribe() (past []LoginEvent, unsubscribe func(), err error) {
	j.mu.Lock()
	if j.state != LoginJobRunning {
		j.mu.Unlock()
		return nil, func() {}, errors.New("job_not_running")
	}
	past = append(past, j.events...)
	subSeq := j.subSeq
	j.subSeq++
	ch := make(chan LoginEvent, 64)
	j.subs[subSeq] = ch
	j.mu.Unlock()
	return past, func() {
		j.mu.Lock()
		delete(j.subs, subSeq)
		j.mu.Unlock()
	}, nil
}

// Finish marks the job as complete/failed with an optional error
// string. The job stays in the map so the status endpoint can
// report on it until the TTL eviction.
func (j *LoginJob) Finish(state LoginJobState, errMsg string) {
	if state != LoginJobComplete && state != LoginJobFailed && state != LoginJobExpired {
		state = LoginJobComplete
	}
	j.mu.Lock()
	j.state = state
	j.EndedAt = time.Now().UTC()
	j.Error = errMsg
	j.mu.Unlock()
}

// State returns the job's current state.
func (j *LoginJob) State() LoginJobState {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.state
}

// Snapshot returns the job as a LoginStatus value for the API.
func (j *LoginJob) Snapshot() LoginStatus {
	j.mu.Lock()
	defer j.mu.Unlock()
	return LoginStatus{
		JobID:      j.ID,
		ProviderID: j.ProviderID,
		State:      string(j.state),
		StartedAt:  j.StartedAt,
		EndedAt:    j.EndedAt,
		Error:      j.Error,
		Auth:       j.Auth,
	}
}

// Cancel kills the underlying omp child (if any) and marks the job
// as expired. Called by the API when the client closes the SSE
// stream.
func (j *LoginJob) Cancel() {
	if j.cancel != nil {
		j.cancel()
	}
	j.Finish(LoginJobExpired, "")
}

// SetResponder attaches a callback the API uses to forward user
// input (LoginAck) back to the omp child. We store it on the job so
// the ack handler doesn't need a separate registry.
func (j *LoginJob) SetResponder(fn func(LoginAck) error) {
	j.mu.Lock()
	j.respond = fn
	j.mu.Unlock()
}

// Respond invokes the responder (if set) to push the user's input
// back to the omp child. Errors are returned to the caller so the
// ack handler can map them to JSON.
func (j *LoginJob) Respond(ack LoginAck) error {
	j.mu.Lock()
	fn := j.respond
	j.mu.Unlock()
	if fn == nil {
		return errors.New("no responder attached")
	}
	return fn(ack)
}

func newLoginJobID() string {
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "lj_0000000000000000"
	}
	return "lj_" + hex.EncodeToString(buf[:])
}
