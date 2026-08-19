package clitools

// In-memory job registry. Mirrors the reference's
// lib/cli-tools/jobs.ts (Map<jobId, CliJob>); in-flight install
// or login jobs die when the api restarts.

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

// Jobs is the in-memory store of live install/login jobs.
// Safe for concurrent use; bounded by a max slot count so a
// buggy client can't leak infinite jobs.
type Jobs struct {
	mu      sync.Mutex
	jobs    map[string]*Job
	maxJobs int
}

// NewJobs returns a fresh store with a sensible default cap.
func NewJobs() *Jobs {
	return &Jobs{
		jobs:    make(map[string]*Job),
		maxJobs: 64,
	}
}

// jobTTL controls when jobs are eligible for GC. The runner
// closes the job (sets State to done/failed) on completion,
// and the Sweep removes anything that ended more than jobTTL
// ago.
const jobTTL = 30 * time.Minute

// NewJob registers a fresh job and returns it. The caller
// fills in Pid / child / cancel before returning to the
// handler.
func (s *Jobs) NewJob(cliID string, kind JobKind) *Job {
	id := newJobID()
	j := &Job{
		ID:        id,
		CliID:     cliID,
		Kind:      kind,
		StartedAt: time.Now().Unix(),
		Status:    JobRunning,
		Lines:     make([]string, 0, ringCap),
		subs:      make(map[int]chan struct{}),
		done:      make(chan struct{}),
	}
	s.mu.Lock()
	if len(s.jobs) >= s.maxJobs {
		s.evictLocked()
	}
	s.jobs[id] = j
	s.mu.Unlock()
	return j
}

// Get returns the job by id, or nil if not found. Sweeps
// stale entries as a side-effect so the map doesn't grow
// unbounded across long uptime.
func (s *Jobs) Get(id string) *Job {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.evictLocked()
	return s.jobs[id]
}

// Sweep removes jobs whose last activity was older than the
// TTL. Exposed for the handler to call periodically.
func (s *Jobs) Sweep() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.evictLocked()
}

func (s *Jobs) evictLocked() {
	cutoff := time.Now().Add(-jobTTL)
	for id, j := range s.jobs {
		if j.Status == JobRunning {
			continue
		}
		if time.Unix(j.StartedAt, 0).Before(cutoff) {
			delete(s.jobs, id)
		}
	}
}

// newJobID is an 8-byte hex id prefixed with "cj_" (cli job).
// crypto/rand rather than monotonic so jobs are not guessable
// across the api process.
func newJobID() string {
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "cj_0000000000000000"
	}
	return "cj_" + hex.EncodeToString(buf[:])
}

// MarkDone closes the job and records the exit code.
func (j *Job) MarkDone(exitCode int) {
	j.muLines.Lock()
	j.Status = JobDone
	j.ExitCode = &exitCode
	j.muLines.Unlock()
	close(j.done)
	j.notify()
}

// MarkFailed closes the job and records an error message.
func (j *Job) MarkFailed(msg string) {
	j.muLines.Lock()
	j.Status = JobFailed
	if msg != "" {
		j.Lines = appendLineLocked(j.Lines, "error: "+msg)
	}
	j.muLines.Unlock()
	close(j.done)
	j.notify()
}

// appendLine pushes a new line onto the ring buffer. Older
// lines drop off once ringCap is reached. Caller must hold
// j.muLines.
func appendLineLocked(buf []string, line string) []string {
	if len(buf) >= ringCap {
		buf = buf[1:]
	}
	buf = append(buf, line)
	return buf
}

// AppendLine is the package-level wrapper used by the runner.
func (j *Job) AppendLine(line string) {
	j.muLines.Lock()
	j.Lines = appendLineLocked(j.Lines, line)
	j.muLines.Unlock()
}

// SetAuth records the device-code URL + code captured by the
// runner's regex pass. Both fields are stable once set; the
// runner's first match wins (some providers repeat the
// prompt).
func (j *Job) SetAuth(url, code string) {
	j.muLines.Lock()
	if j.AuthURL == "" && url != "" {
		j.AuthURL = url
	}
	if j.AuthCode == "" && code != "" {
		j.AuthCode = code
	}
	j.muLines.Unlock()
}

// Done returns a channel that is closed when the job finishes.
func (j *Job) Done() <-chan struct{} { return j.done }

// Snapshot returns a stable copy of the job's state plus the
// buffered lines. Safe for concurrent use.
func (j *Job) Snapshot() JobSnapshot {
	j.muLines.Lock()
	defer j.muLines.Unlock()
	lines := append([]string(nil), j.Lines...)
	return JobSnapshot{
		ID:        j.ID,
		CliID:     j.CliID,
		Kind:      j.Kind,
		Status:    j.Status,
		StartedAt: j.StartedAt,
		ExitCode:  j.ExitCode,
		Lines:     lines,
		AuthURL:   j.AuthURL,
		AuthCode:  j.AuthCode,
	}
}

// JobSnapshot is the read-only view of a Job for HTTP handlers
// and SSE subscribers.
type JobSnapshot struct {
	ID        string
	CliID     string
	Kind      JobKind
	Status    JobState
	StartedAt int64
	ExitCode  *int
	Lines     []string
	AuthURL   string
	AuthCode  string
}


// Subscribe registers a channel that pings whenever the job
// emits a new event, and returns an unsubscribe func. Used
// by the SSE handler to wake up on log/status changes.
func (j *Job) Subscribe() (chan struct{}, func()) {
	ch := make(chan struct{}, 64)
	j.muSub.Lock()
	id := j.subSeq
	j.subSeq++
	j.subs[id] = ch
	j.muSub.Unlock()
	return ch, func() {
		j.muSub.Lock()
		delete(j.subs, id)
		j.muSub.Unlock()
	}
}

// notify pings any SSE subscribers so they re-poll the job
// state.
func (j *Job) notify() {
	j.muSub.Lock()
	for _, ch := range j.subs {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
	j.muSub.Unlock()
}