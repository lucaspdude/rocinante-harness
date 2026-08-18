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
	"io"
	"sync"
	"time"
)

// LoginJobState is the lifecycle state of a single login attempt.
type LoginJobState string

const (
	LoginJobRunning  LoginJobState = "running"
	LoginJobComplete LoginJobState = "complete"
	LoginJobFailed   LoginJobState = "failed"
	LoginJobExpired  LoginJobState = "expired"
)

// LoginEvent is a single SSE event emitted by the omp child.
type LoginEvent struct {
	Event string
	Data  map[string]any
}

// LoginJob is one in-flight login attempt.
type LoginJob struct {
	ID         string
	ProviderID string
	StartedAt  time.Time
	EndedAt    time.Time
	Error      string

	mu      sync.Mutex
	state   LoginJobState
	subSeq  int
	events  []LoginEvent
	subs    map[int]chan LoginEvent
	cancel  context.CancelFunc
	respond func(LoginAck) error
	stdin   io.WriteCloser
}

// LoginJobs is the in-memory store of live login jobs.
type LoginJobs struct {
	mu      sync.Mutex
	jobs    map[string]*LoginJob
	maxJobs int
}

// NewLoginJobs returns a fresh login-job store.
func NewLoginJobs() *LoginJobs {
	return &LoginJobs{
		jobs:    make(map[string]*LoginJob),
		maxJobs: 32,
	}
}

// NewJob creates a LoginJob with a fresh id and an event channel.
func (s *LoginJobs) NewJob(providerID string, cancel context.CancelFunc) *LoginJob {
	id := newLoginJobID()
	now := time.Now().UTC()
	j := &LoginJob{
		ID:         id,
		ProviderID: providerID,
		StartedAt:  now,
		state:      LoginJobRunning,
		subs:       make(map[int]chan LoginEvent),
		cancel:     cancel,
	}
	s.mu.Lock()
	if len(s.jobs) >= s.maxJobs {
		s.evictLocked()
	}
	s.jobs[id] = j
	s.mu.Unlock()
	return j
}

// Sweep removes jobs older than the TTL regardless of state.
func (s *LoginJobs) Sweep() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.evictLocked()
}

func (s *LoginJobs) evictLocked() {
	cutoff := time.Now().Add(-time.Hour)
	for id, j := range s.jobs {
		j.mu.Lock()
		ended := !j.EndedAt.IsZero() && j.EndedAt.Before(cutoff)
		j.mu.Unlock()
		if ended {
			delete(s.jobs, id)
		}
	}
}

// Get returns the job by id, or nil.
func (s *LoginJobs) Get(id string) *LoginJob {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.jobs[id]
}

func newLoginJobID() string {
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "lj_0000000000000000"
	}
	return "lj_" + hex.EncodeToString(buf[:])
}

// publish appends an event to the history and fans it out to every
// active subscriber.
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
		}
	}
}

// Subscribe registers a channel that receives future events and
// returns the past events as a snapshot.
func (j *LoginJob) Subscribe() (past []LoginEvent, unsubscribe func(), err error) {
	j.mu.Lock()
	if j.state != LoginJobRunning {
		j.mu.Unlock()
		return nil, func() {}, errors.New("job_not_running")
	}
	past = append(past, j.events...)
	sub := make(chan LoginEvent, 64)
	subSeq := j.subSeq
	j.subSeq++
	j.subs[subSeq] = sub
	j.mu.Unlock()
	return past, func() {
		j.mu.Lock()
		delete(j.subs, subSeq)
		j.mu.Unlock()
	}, nil
}

// Finish marks the job as complete/failed.
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

// Snapshot returns the job as a LoginStatus value.
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
	}
}

// Cancel kills the underlying omp child (if any).
func (j *LoginJob) Cancel() {
	if j.cancel != nil {
		j.cancel()
	}
	j.Finish(LoginJobExpired, "")
}

// SetResponder attaches a callback the API uses to forward user
// input (LoginAck) back to the omp child.
func (j *LoginJob) SetResponder(fn func(LoginAck) error) {
	j.mu.Lock()
	j.respond = fn
	j.mu.Unlock()
}

// SetStdin wires the omp child's stdin write end so the responder
// can write extension_ui_response frames back to the child.
func (j *LoginJob) SetStdin(w io.WriteCloser) {
	j.mu.Lock()
	j.stdin = w
	j.mu.Unlock()
}

// Stdin returns the job's stdin write end (or nil).
func (j *LoginJob) Stdin() io.WriteCloser {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.stdin
}

// Respond invokes the responder (if set).
func (j *LoginJob) Respond(ack LoginAck) error {
	j.mu.Lock()
	fn := j.respond
	j.mu.Unlock()
	if fn == nil {
		return errors.New("no responder attached")
	}
	return fn(ack)
}
