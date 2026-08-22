package sessions

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

// TestReplayHandler_ReturnsEntries writes two entries and verifies
// the handler returns them as JSON.
func TestReplayHandler_ReturnsEntries(t *testing.T) {
	dir := t.TempDir()
	store := New(dir)

	store.Append("sess", Entry{Kind: "message", Message: json.RawMessage(`{"id":"m1","role":"user"}`)})
	store.Append("sess", Entry{Kind: "frame", Seq: intPtr(7), Frame: json.RawMessage(`{"type":"delta","seq":7,"text":"x"}`)})

	r := chi.NewRouter()
	r.Get("/api/v1/sessions/{id}/messages", ReplayHandler(store))
	srv := httptest.NewServer(r)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/v1/sessions/sess/messages")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var body ReplayResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.ID != "sess" {
		t.Errorf("id = %q, want sess", body.ID)
	}
	if len(body.Entries) != 2 {
		t.Fatalf("entries len = %d, want 2", len(body.Entries))
	}
	if body.Entries[0].Kind != "message" {
		t.Errorf("entries[0].kind = %q, want message", body.Entries[0].Kind)
	}
	if body.Entries[1].Kind != "frame" || body.Entries[1].Seq == nil || *body.Entries[1].Seq != 7 {
		t.Errorf("entries[1] = %+v, want frame seq=7", body.Entries[1])
	}
}

// TestReplayHandler_EmptySession returns 200 with empty entries
// when the session has never been written. The web relies on this
// to bootstrap a fresh session without a 404 flicker.
func TestReplayHandler_EmptySession(t *testing.T) {
	dir := t.TempDir()
	store := New(dir)

	r := chi.NewRouter()
	r.Get("/api/v1/sessions/{id}/messages", ReplayHandler(store))
	srv := httptest.NewServer(r)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/v1/sessions/fresh/messages")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var body ReplayResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Entries) != 0 {
		t.Errorf("entries = %d, want 0", len(body.Entries))
	}
}

// TestReplayHandler_SinceHonored checks that the since=N query
// param skips the first N lines (reconnect-tail contract).
func TestReplayHandler_SinceHonored(t *testing.T) {
	dir := t.TempDir()
	store := New(dir)

	for i := 0; i < 3; i++ {
		seq := i + 1
		store.Append("sess", Entry{Kind: "frame", Seq: &seq, Frame: json.RawMessage(`{"type":"delta"}`)})
	}

	r := chi.NewRouter()
	r.Get("/api/v1/sessions/{id}/messages", ReplayHandler(store))
	srv := httptest.NewServer(r)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/v1/sessions/sess/messages?since=2")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	var body ReplayResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(body.Entries))
	}
	if body.Entries[0].Seq == nil || *body.Entries[0].Seq != 3 {
		t.Errorf("entries[0].seq = %v, want 3", body.Entries[0].Seq)
	}
}

// TestReplayHandler_BadSince returns 400 when since is non-numeric.
func TestReplayHandler_BadSince(t *testing.T) {
	dir := t.TempDir()
	store := New(dir)

	r := chi.NewRouter()
	r.Get("/api/v1/sessions/{id}/messages", ReplayHandler(store))
	srv := httptest.NewServer(r)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/v1/sessions/sess/messages?since=notanumber")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}
