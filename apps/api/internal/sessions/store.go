// Package sessions persists per-session chat history as a JSONL log
// under $SHARE_DIR/sessions/{id}/messages.jsonl. PR-2 (phase 6) ships
// this so the web client can rehydrate a chat from disk after a
// reload, instead of starting empty.
package sessions

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
)

// MaxLogBytes caps the JSONL log per session. When a write would push
// the file past this size, the oldest lines are dropped (head
// truncation) so the file never grows unbounded.
const MaxLogBytes = 10 * 1024 * 1024

// keepLines is the safety floor when truncating — never drop below
// this many lines so the user keeps a reasonable window of context
// even after a very long session.
const keepLines = 256

// Entry is one line in the JSONL log. The Kind discriminator tells
// the consumer whether this row is a chat message (written on prompt
// POST) or a raw SSE frame (written as it streams). The JSONL is
// append-only between truncations.
type Entry struct {
	Kind    string          `json:"kind"`              // "message" or "frame"
	Seq     *int            `json:"seq,omitempty"`     // SSE seq, used for dedup
	Frame   json.RawMessage `json:"frame,omitempty"`   // raw SSE payload (kind=frame)
	Message json.RawMessage `json:"message,omitempty"` // ChatMessage payload (kind=message)
}

// Store is a JSONL-backed per-session log. Safe for concurrent use
// across goroutines (a session's prompt handler and stream handler
// can write from different goroutines).
type Store struct {
	shareDir string
	mu       sync.Mutex
}

// New returns a Store rooted at shareDir. The actual log file is
// created lazily on the first Write call for a given session id.
func New(shareDir string) *Store {
	return &Store{shareDir: shareDir}
}

// logPath returns the absolute path of the JSONL log for the given
// session id.
func (s *Store) logPath(id string) string {
	return filepath.Join(s.shareDir, "sessions", id, "messages.jsonl")
}

// ensureDir creates the session directory with 0700 permissions so
// other users on the host can't read the chat log.
func (s *Store) ensureDir(id string) error {
	dir := filepath.Join(s.shareDir, "sessions", id)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("mkdir session dir: %w", err)
	}
	// MkdirAll honors the umask; re-chmod to enforce 0700 in case
	// the directory already existed with looser permissions.
	if err := os.Chmod(dir, 0o700); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("chmod session dir: %w", err)
	}
	return nil
}

// Append serializes entry as JSON, writes one line to the log file
// (atomic per line via O_APPEND), and triggers a head-truncation if
// the log exceeds MaxLogBytes. Errors are logged but not returned:
// persistence is best-effort; a write failure must not break the
// user's chat.
func (s *Store) Append(id string, entry Entry) {
	if s == nil || id == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureDir(id); err != nil {
		log.Printf("sessions: ensure dir for %s: %v", id, err)
		return
	}
	payload, err := json.Marshal(entry)
	if err != nil {
		log.Printf("sessions: marshal entry for %s: %v", id, err)
		return
	}
	payload = append(payload, '\n')
	path := s.logPath(id)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		log.Printf("sessions: open log for %s: %v", id, err)
		return
	}
	if _, err := f.Write(payload); err != nil {
		_ = f.Close()
		log.Printf("sessions: write log for %s: %v", id, err)
		return
	}
	if err := f.Close(); err != nil {
		log.Printf("sessions: close log for %s: %v", id, err)
		return
	}
	s.maybeTruncate(path)
}

// maybeTruncate checks the file size; if it exceeds MaxLogBytes,
// rewrites the tail back to the same path. Called under s.mu.
//
// Drop semantics: each iteration computes the size of the kept
// window and drops one line from the head while (size > MaxLogBytes
// AND we still have more than one line to keep). The keepLines
// floor is intentionally NOT applied here: a hard cap is more
// important than preserving a fixed window — when entries are small
// the floor is irrelevant (we never reach it), and when entries
// are huge we'd violate the cap anyway.
func (s *Store) maybeTruncate(path string) {
	fi, err := os.Stat(path)
	if err != nil || fi.Size() <= MaxLogBytes {
		return
	}
	lines, err := readAllLines(path)
	if err != nil {
		log.Printf("sessions: read all for truncation: %v", err)
		return
	}
	dropped := 0
	for {
		size := totalLineSize(lines)
		if size <= MaxLogBytes {
			break
		}
		if len(lines) <= 1 {
			break
		}
		lines = lines[1:]
		dropped++
	}
	if dropped == 0 {
		// Nothing dropped — a single oversized entry made the cap
		// impossible. Skip the rewrite; the next append will
		// retry, and once a smaller entry arrives we'll catch up.
		return
	}
	tmp := path + ".tmp"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		log.Printf("sessions: open tmp for truncation: %v", err)
		return
	}
	w := bufio.NewWriter(out)
	for _, line := range lines {
		if _, err := w.Write(line); err != nil {
			_ = out.Close()
			log.Printf("sessions: write tmp for truncation: %v", err)
			return
		}
		if _, err := w.Write([]byte{'\n'}); err != nil {
			_ = out.Close()
			log.Printf("sessions: write newline for truncation: %v", err)
			return
		}
	}
	if err := w.Flush(); err != nil {
		_ = out.Close()
		log.Printf("sessions: flush tmp for truncation: %v", err)
		return
	}
	if err := out.Close(); err != nil {
		log.Printf("sessions: close tmp for truncation: %v", err)
		return
	}
	if err := os.Rename(tmp, path); err != nil {
		log.Printf("sessions: rename tmp for truncation: %v", err)
	}
}

// totalLineSize returns the on-disk size of lines (bytes + one
// newline per line). Used by maybeTruncate to decide how many lines
// to drop from the head.
func totalLineSize(lines [][]byte) int64 {
	var size int64
	for _, l := range lines {
		size += int64(len(l)) + 1
	}
	return size
}

// readAllLines returns every non-empty line of path, preserving the
// original byte content of each line (no trailing newline). Used by
// maybeTruncate so we can size-check the kept window before
// rewriting.
func readAllLines(path string) ([][]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), MaxLogBytes)
	var all [][]byte
	for scanner.Scan() {
		b := append([]byte(nil), scanner.Bytes()...)
		if len(b) == 0 {
			continue
		}
		all = append(all, b)
	}
	if err := scanner.Err(); err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	return all, nil
}

// Replay returns the persisted entries for the session, optionally
// skipping the first `since` lines (used for reconnect-tail). Lines
// that fail to JSON-parse are skipped with a warn log; a corrupt
// line must not break the session.
func (s *Store) Replay(id string, since int) ([]Entry, error) {
	if s == nil || id == "" {
		return nil, os.ErrNotExist
	}
	path := s.logPath(id)
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, os.ErrNotExist
		}
		return nil, err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), MaxLogBytes)
	var out []Entry
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		if lineNum <= since {
			continue
		}
		raw := scanner.Bytes()
		if len(raw) == 0 {
			continue
		}
		var entry Entry
		if err := json.Unmarshal(raw, &entry); err != nil {
			log.Printf("sessions: skip corrupt line %d for %s: %v", lineNum, id, err)
			continue
		}
		out = append(out, entry)
	}
	if err := scanner.Err(); err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	return out, nil
}

// LineCount returns the number of persisted lines for the session.
// Used by tests; not exposed via the HTTP API.
func (s *Store) LineCount(id string) (int, error) {
	if s == nil || id == "" {
		return 0, nil
	}
	path := s.logPath(id)
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, nil
		}
		return 0, err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), MaxLogBytes)
	n := 0
	for scanner.Scan() {
		if len(scanner.Bytes()) > 0 {
			n++
		}
	}
	return n, scanner.Err()
}
