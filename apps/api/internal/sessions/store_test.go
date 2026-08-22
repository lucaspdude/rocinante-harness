package sessions

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestStore_WriteReplayRoundTrip writes a handful of mixed
// messages + frames, replays them, and verifies order + content.
func TestStore_WriteReplayRoundTrip(t *testing.T) {
	dir := t.TempDir()
	store := New(dir)

	userMsg := json.RawMessage(`{"id":"m1","role":"user","content":"hello","createdAt":"2026-08-22T10:00:00Z"}`)
	frame := json.RawMessage(`{"type":"delta","seq":1,"text":"hi"}`)
	frame2 := json.RawMessage(`{"type":"agent_end","seq":2}`)

	store.Append("sess-a", Entry{Kind: "message", Message: userMsg})
	store.Append("sess-a", Entry{Kind: "frame", Seq: intPtr(1), Frame: frame})
	store.Append("sess-a", Entry{Kind: "frame", Seq: intPtr(2), Frame: frame2})

	got, err := store.Replay("sess-a", 0)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("replay returned %d entries, want 3", len(got))
	}
	if got[0].Kind != "message" || string(got[0].Message) != string(userMsg) {
		t.Errorf("entry[0] = %+v, want message/userMsg", got[0])
	}
	if got[1].Kind != "frame" || got[1].Seq == nil || *got[1].Seq != 1 {
		t.Errorf("entry[1] = %+v, want frame seq=1", got[1])
	}
	if got[2].Kind != "frame" || got[2].Seq == nil || *got[2].Seq != 2 {
		t.Errorf("entry[2] = %+v, want frame seq=2", got[2])
	}

	// LineCount should reflect the same.
	n, err := store.LineCount("sess-a")
	if err != nil {
		t.Fatalf("line count: %v", err)
	}
	if n != 3 {
		t.Errorf("LineCount = %d, want 3", n)
	}
}

// TestStore_ReplaySince skips the first N lines so reconnect
// clients can resume from an offset without re-reading the entire
// log.
func TestStore_ReplaySince(t *testing.T) {
	dir := t.TempDir()
	store := New(dir)

	for i := 0; i < 5; i++ {
		seq := i + 1
		store.Append("sess-b", Entry{
			Kind:  "frame",
			Seq:   &seq,
			Frame: json.RawMessage(`{"type":"status"}`),
		})
	}

	full, err := store.Replay("sess-b", 0)
	if err != nil {
		t.Fatalf("replay full: %v", err)
	}
	if len(full) != 5 {
		t.Fatalf("full replay returned %d, want 5", len(full))
	}

	tail, err := store.Replay("sess-b", 2)
	if err != nil {
		t.Fatalf("replay since=2: %v", err)
	}
	if len(tail) != 3 {
		t.Errorf("since=2 replay returned %d, want 3", len(tail))
	}
	if tail[0].Seq == nil || *tail[0].Seq != 3 {
		t.Errorf("tail[0].Seq = %v, want 3", tail[0].Seq)
	}
}

// TestStore_ReplaySkipsCorruptLines proves that a corrupt line in
// the log does not break the replay — the line is skipped with a
// warn log (we don't assert the log; just that valid neighbors
// survive).
func TestStore_ReplaySkipsCorruptLines(t *testing.T) {
	dir := t.TempDir()
	store := New(dir)

	store.Append("sess-c", Entry{Kind: "message", Message: json.RawMessage(`{"id":"a"}`)})

	// Append a corrupt line directly to the file.
	if err := os.MkdirAll(filepath.Join(dir, "sessions", "sess-c"), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	logPath := filepath.Join(dir, "sessions", "sess-c", "messages.jsonl")
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if _, err := f.WriteString("this is not json\n"); err != nil {
		t.Fatalf("write corrupt: %v", err)
	}
	if _, err := f.WriteString("{\"kind\":\"message\",\"message\":{\"id\":\"b\"}}\n"); err != nil {
		t.Fatalf("write good: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	got, err := store.Replay("sess-c", 0)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("replay returned %d entries, want 2 (corrupt skipped)", len(got))
	}
	if string(got[0].Message) != `{"id":"a"}` {
		t.Errorf("got[0].Message = %s, want {id:a}", string(got[0].Message))
	}
	if string(got[1].Message) != `{"id":"b"}` {
		t.Errorf("got[1].Message = %s, want {id:b}", string(got[1].Message))
	}
}

// TestStore_ReplayMissingSession returns ErrNotExist when the log
// file has never been written; the HTTP layer maps this to 200 with
// empty entries.
func TestStore_ReplayMissingSession(t *testing.T) {
	dir := t.TempDir()
	store := New(dir)
	_, err := store.Replay("never-existed", 0)
	if !os.IsNotExist(err) {
		t.Errorf("Replay on missing session = %v, want ErrNotExist", err)
	}
	if n, _ := store.LineCount("never-existed"); n != 0 {
		t.Errorf("LineCount on missing = %d, want 0", n)
	}
}

// TestStore_EnsureDirIs0700 verifies the directory permission
// guard so other users on the host can't read chat logs.
func TestStore_EnsureDirIs0700(t *testing.T) {
	dir := t.TempDir()
	store := New(dir)

	store.Append("sess-d", Entry{Kind: "message", Message: json.RawMessage(`{}`)})

	info, err := os.Stat(filepath.Join(dir, "sessions", "sess-d"))
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o700 {
		t.Errorf("session dir perms = %o, want 0700", perm)
	}
}

// TestStore_TruncatesHeadOnOversize exercises the 10MB cap. Six
// appends of ~2MB each cross the cap multiple times; the truncator
// must drop entries from the head so the file stays under the cap
// (with a single-entry slack for the just-appended line).
func TestStore_TruncatesHeadOnOversize(t *testing.T) {
	dir := t.TempDir()
	store := New(dir)

	big := strings.Repeat("x", 2*1024*1024)
	for i := 0; i < 6; i++ {
		store.Append("sess-e", Entry{
			Kind:    "message",
			Message: json.RawMessage(`{"blob":"` + big + `"}`),
		})
	}

	fi, err := os.Stat(filepath.Join(dir, "sessions", "sess-e", "messages.jsonl"))
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	// After the last append, the truncator dropped at least one
	// line, so the file should be ≤ MaxLogBytes plus a single
	// entry of slack (the line just appended).
	if fi.Size() > MaxLogBytes+int64(len(big))+1024 {
		t.Errorf("log size = %d, expected ≤ %d + slack", fi.Size(), MaxLogBytes)
	}
	// And the line count proves the truncation actually fired:
	// 6 appends but at least one was dropped, so ≤ 5 remain.
	if n, _ := store.LineCount("sess-e"); n >= 6 {
		t.Errorf("expected head truncation to drop ≥1 line, got %d lines", n)
	}
}

func intPtr(i int) *int { return &i }
