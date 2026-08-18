package projects

// Git-clone orchestration. PR-04 adds the SSE stream of git-clone
// progress and registers the resulting repo with the
// ProjectRegistry. SSH URLs (git@github.com:...) are rejected
// until the SSH UI lands (Phase 3).

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
)

// CloneRequest is the body of POST /api/v1/projects/clone.
type CloneRequest struct {
	URL        string `json:"url"`
	ParentPath string `json:"parent_path"`
	FolderName string `json:"folder_name,omitempty"`
}

// Validate enforces the URL + folder constraints required before
// spawning the git child.
func (r *CloneRequest) Validate() error {
	if r.URL == "" {
		return errors.New("url required")
	}
	if r.ParentPath == "" {
		return errors.New("parent_path required")
	}
	if looksLikeSSH(r.URL) {
		return errors.New("ssh_keys_missing")
	}
	if !validURL(r.URL) {
		return errors.New("invalid_url")
	}
	if r.FolderName == "" {
		r.FolderName = deriveFolderName(r.URL)
	}
	if !validFolderName(r.FolderName) {
		return errors.New("invalid_folder_name")
	}
	return nil
}

var sshPattern = regexp.MustCompile(`^[\w.-]+@[\w.-]+:`)
var urlPattern = regexp.MustCompile(`^(https://|git://|ssh://)?[\w.\-]+(:[0-9]+)?(/[^\s]*)?$`)

func looksLikeSSH(u string) bool {
	return sshPattern.MatchString(u)
}

func validURL(u string) bool {
	if strings.Contains(u, "..") {
		return false
	}
	if _, err := url.Parse(u); err == nil &&
		(strings.HasPrefix(u, "https://") || strings.HasPrefix(u, "git://") || strings.HasPrefix(u, "ssh://")) {
		return true
	}
	if strings.Count(u, "/") == 1 && !strings.Contains(u, " ") {
		return true
	}
	return urlPattern.MatchString(u)
}

var folderNamePattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

func validFolderName(name string) bool {
	if name == "" || name == "." || name == ".." {
		return false
	}
	return folderNamePattern.MatchString(name)
}

func deriveFolderName(rawURL string) string {
	u := rawURL
	u = strings.TrimSuffix(u, ".git")
	if i := strings.IndexAny(u, "?#"); i >= 0 {
		u = u[:i]
	}
	if i := strings.LastIndex(u, "/"); i >= 0 {
		u = u[i+1:]
	}
	if i := strings.LastIndex(u, ":"); i >= 0 {
		u = u[i+1:]
	}
	return u
}

// CloneEvent is one SSE frame pushed to subscribers.
type CloneEvent struct {
	Event string         // "log" | "progress" | "phase" | "complete" | "fail" | "registered"
	Data  map[string]any
}

// CloneJob holds the live clone state.
type CloneJob struct {
	ID         string
	URL        string
	ParentPath string
	FolderName string
	CreatedAt  time.Time
	EndedAt    time.Time
	State      string // "running" | "complete" | "failed"
	Error      string

	mu     sync.Mutex
	subs   map[int]chan CloneEvent
	evBuf  []CloneEvent
	done   chan struct{}
	cancel context.CancelFunc
}

// CloneJobs is the in-memory store. Bounded by clone-jobs TTL.
type CloneJobs struct {
	mu      sync.Mutex
	jobs    map[string]*CloneJob
	maxJobs int
}

// NewCloneJobs returns a fresh job store.
func NewCloneJobs() *CloneJobs {
	return &CloneJobs{
		jobs:    make(map[string]*CloneJob),
		maxJobs: 64,
	}
}

// All returns a snapshot of currently tracked jobs (best-effort).
func (s *CloneJobs) All() []*CloneJob {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*CloneJob, 0, len(s.jobs))
	for _, j := range s.jobs {
		out = append(out, j)
	}
	return out
}

var cloneIDSeq uint64

func newCloneID() string {
	const base36 = "0123456789abcdefghijklmnopqrstuvwxyz"
	cloneIDSeq++
	x := cloneIDSeq
	if x == 0 {
		x = uint64(time.Now().UnixNano())
	}
	out := "cl_"
	for x > 0 {
		out += string(base36[x%36])
		x /= 36
	}
	return out
}

// NewJob registers a fresh job and returns it.
func (s *CloneJobs) NewJob(req CloneRequest) *CloneJob {
	id := newCloneID()
	now := time.Now().UTC()
	j := &CloneJob{
		ID:         id,
		URL:        req.URL,
		ParentPath: req.ParentPath,
		FolderName: req.FolderName,
		CreatedAt:  now,
		State:      "running",
		subs:       make(map[int]chan CloneEvent),
		done:       make(chan struct{}),
	}
	s.mu.Lock()
	if len(s.jobs) >= s.maxJobs {
		s.evictLocked()
	}
	s.jobs[id] = j
	s.mu.Unlock()
	return j
}

// Sweep removes jobs older than the TTL (default 1h).
func (s *CloneJobs) Sweep() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.evictLocked()
}

func (s *CloneJobs) evictLocked() {
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
func (s *CloneJobs) Get(id string) *CloneJob {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.jobs[id]
}



// Subscribe registers a channel that receives future events.
func (j *CloneJob) Subscribe() (past []CloneEvent, unsubscribe func()) {
	j.mu.Lock()
	if j.State != "running" {
		j.mu.Unlock()
		return nil, func() {}
	}
	past = append(past, j.evBuf...)
	sub := make(chan CloneEvent, 64)
	id := len(j.subs) + 1
	j.subs[id] = sub
	j.mu.Unlock()
	return past, func() {
		j.mu.Lock()
		delete(j.subs, id)
		j.mu.Unlock()
	}
}

// publish appends an event to the buffer and fans out to subscribers.
func (j *CloneJob) publish(ev CloneEvent) {
	j.mu.Lock()
	j.evBuf = append(j.evBuf, ev)
	subs := make([]chan CloneEvent, 0, len(j.subs))
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

// Snapshot returns the job's current state for the status endpoint.
func (j *CloneJob) Snapshot() CloneStatus {
	j.mu.Lock()
	defer j.mu.Unlock()
	return CloneStatus{
		JobID:      j.ID,
		URL:        j.URL,
		ParentPath: j.ParentPath,
		FolderName: j.FolderName,
		State:      j.State,
		CreatedAt:  j.CreatedAt,
		EndedAt:    j.EndedAt,
		Error:      j.Error,
	}
}

// CloneStatus is the public read-only view of a CloneJob.
type CloneStatus struct {
	JobID      string
	URL        string
	ParentPath string
	FolderName string
	State      string
	CreatedAt  time.Time
	EndedAt    time.Time
	Error      string
}

// Cancel stops the clone child (if any) and closes the job.
func (j *CloneJob) Cancel() {
	if j.cancel != nil {
		j.cancel()
	}
}

// Run drives the git clone child, parses stderr, and emits events.
func (j *CloneJob) Run(ctx context.Context, bin string, reg *Registry, fileAccess FileAccessAllow, onComplete func(path string) error) {
	defer close(j.done)

	j.mu.Lock()
	ctx2, cancel := context.WithTimeout(ctx, 10*time.Minute)
	j.cancel = cancel
	j.mu.Unlock()

	target := joinPath(j.ParentPath, j.FolderName)
	j.publish(CloneEvent{Event: "phase", Data: map[string]any{
		"phase":   "starting",
		"command": []string{bin, "clone", j.URL, target},
	}})

	cmd := execCommandContext(ctx2, bin, "clone", "--progress", j.URL, target)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		j.fail("pipe: " + err.Error())
		return
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		j.fail("stderr pipe: " + err.Error())
		return
	}
	if err := cmd.Start(); err != nil {
		j.fail("spawn: " + err.Error())
		return
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		j.scanLines(stdout, "stdout")
	}()
	go func() {
		defer wg.Done()
		j.scanLines(stderr, "stderr")
	}()
	wg.Wait()
	if err := cmd.Wait(); err != nil {
		if errors.Is(ctx2.Err(), context.DeadlineExceeded) {
			j.fail("clone_failed: timeout after 10m")
			return
		}
		j.fail("clone_failed: " + err.Error())
		return
	}
	j.publish(CloneEvent{Event: "phase", Data: map[string]any{
		"phase": "complete",
		"path":  target,
	}})
	if reg != nil {
		p, err := reg.Upsert(target, j.FolderName, "", false)
		if err != nil {
			j.fail("register_failed: " + err.Error())
			return
		}
		j.publish(CloneEvent{Event: "registered", Data: map[string]any{
			"project": p,
		}})
	}
	if fileAccess != nil {
		_ = fileAccess.Allow(target)
	}
	if onComplete != nil {
		if err := onComplete(target); err != nil {
			j.fail("callback_failed: " + err.Error())
			return
		}
	}
	j.mu.Lock()
	j.State = "complete"
	j.EndedAt = time.Now().UTC()
	j.mu.Unlock()
	j.publish(CloneEvent{Event: "complete", Data: map[string]any{"path": target}})
}

// fail marks the job as failed and emits a single fail event.
func (j *CloneJob) fail(msg string) {
	j.mu.Lock()
	j.State = "failed"
	j.EndedAt = time.Now().UTC()
	j.Error = msg
	j.mu.Unlock()
	j.publish(CloneEvent{Event: "fail", Data: map[string]any{"error": msg}})
}

// scanLines reads a pipe line-by-line, parses clone progress, and
// emits events.
func (j *CloneJob) scanLines(r io.Reader, source string) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		pct := parseClonePct(line)
		ev := CloneEvent{Event: "log", Data: map[string]any{
			"source": source,
			"line":   redactLine(line),
		}}
		if pct >= 0 {
			ev.Event = "progress"
			ev.Data["pct"] = pct
		}
		j.publish(ev)
	}
}

// parseClonePct extracts a percentage from a git-clone progress
// line like "Receiving objects:  67% (1234/5678)". Returns -1 if
// no percentage is present.
func parseClonePct(line string) int {
	i := strings.Index(line, "%")
	if i < 0 {
		return -1
	}
	end := i
	for end > 0 && line[end-1] >= '0' && line[end-1] <= '9' {
		end--
	}
	if end == i {
		return -1
	}
	pct := 0
	for _, r := range line[end:i] {
		pct = pct*10 + int(r-'0')
	}
	return pct
}

// redactLine removes basic-auth credentials from URLs in case a
// user pastes an embedded-auth url.
func redactLine(line string) string {
	at := strings.Index(line, "@")
	proto := strings.Index(line, "://")
	if at < 0 || proto < 0 || at <= proto+3 {
		return line
	}
	return line[:proto+3] + "***@" + line[at+1:]
}

// FileAccessAllow is a tiny seam so Run doesn't import the file
// package (which would create a cycle).
type FileAccessAllow interface {
	Allow(root string) error
}

// execCommandContext is a seam so tests can stub exec.CommandContext.
var execCommandContext = func(ctx context.Context, name string, args ...string) cmdIface {
	return defaultCommandContext(ctx, name, args...)
}

type cmdIface interface {
	StdoutPipe() (io.ReadCloser, error)
	StderrPipe() (io.ReadCloser, error)
	Start() error
	Wait() error
}

// joinPath is a small helper to avoid the path/filepath import.
func joinPath(a, b string) string {
	if a == "" {
		return b
	}
	if b == "" {
		return a
	}
	if strings.HasSuffix(a, "/") {
		return a + b
	}
	return a + "/" + b
}
