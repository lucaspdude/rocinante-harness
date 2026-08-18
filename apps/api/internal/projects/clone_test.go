package projects

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"
)

func TestValidateURL(t *testing.T) {
	cases := []struct {
		url    string
		wantOK bool
	}{
		{"https://github.com/foo/bar.git", true},
		{"https://github.com/foo/bar", true},
		{"git@github.com:foo/bar.git", false},
		{"git@github.com:foo/bar", false},
		{"git://github.com/foo/bar.git", true},
		{"owner/repo", true},
		{"", false},
		{"ftp://example.com/foo", false},
		{"/../etc/passwd", false},
		{"https://../escape", false},
	}
	for _, c := range cases {
		req := CloneRequest{URL: c.url, ParentPath: "/tmp"}
		err := req.Validate()
		gotOK := err == nil
		if gotOK != c.wantOK {
			t.Errorf("URL %q: ok=%v want %v (err=%v)", c.url, gotOK, c.wantOK, err)
		}
	}
}

func TestValidateSSHDetection(t *testing.T) {
	req := CloneRequest{URL: "git@host.example:foo/bar.git", ParentPath: "/tmp"}
	err := req.Validate()
	if err == nil || err.Error() != "ssh_keys_missing" {
		t.Errorf("expected ssh_keys_missing; got %v", err)
	}
}

func TestValidFolderName(t *testing.T) {
	cases := []struct {
		name   string
		wantOK bool
	}{
		{"my-repo", true},
		{"repo.git-1", true},
		{"../escape", false},
		{".", false},
		{"..", false},
		{"", false},
		{"with space", false},
		{"semi;colon", false},
	}
	for _, c := range cases {
		got := validFolderName(c.name)
		if got != c.wantOK {
			t.Errorf("validFolderName(%q) = %v want %v", c.name, got, c.wantOK)
		}
	}
}

func TestDeriveFolderName(t *testing.T) {
	cases := map[string]string{
		"https://github.com/foo/bar.git": "bar",
		"https://github.com/foo/bar":     "bar",
		"git@github.com:foo/baz.git":     "baz",
		"owner/repo":                     "repo",
	}
	for in, want := range cases {
		got := deriveFolderName(in)
		if got != want {
			t.Errorf("deriveFolderName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRedactLine(t *testing.T) {
	cases := map[string]string{
		"Cloning into 'https://user:pass@host.example/foo/bar'":
			"Cloning into 'https://***@host.example/foo/bar'",
		"no auth here": "no auth here",
	}
	for in, want := range cases {
		if got := redactLine(in); got != want {
			t.Errorf("redactLine(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseClonePct(t *testing.T) {
	cases := map[string]int{
		"Receiving objects:  67% (1234/5678)": 67,
		"Resolving deltas: 100% (123/123)":   100,
		"no percentage here":                 -1,
		"":                                  -1,
		"hello %world":                      -1,
	}
	for in, want := range cases {
		if got := parseClonePct(in); got != want {
			t.Errorf("parseClonePct(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestCloneJobSweepAndGet(t *testing.T) {
	js := NewCloneJobs()
	j1 := js.NewJob(CloneRequest{URL: "https://x/y.git", ParentPath: "/tmp", FolderName: "y"})
	j2 := js.NewJob(CloneRequest{URL: "https://x/z.git", ParentPath: "/tmp", FolderName: "z"})
	if got := js.Get(j1.ID); got == nil {
		t.Errorf("Get returned nil for j1")
	}
	if got := js.Get(j2.ID); got == nil {
		t.Errorf("Get returned nil for j2")
	}
	if got := js.Get("nope"); got != nil {
		t.Errorf("Get returned non-nil for missing id")
	}
	js.Sweep()
	if len(js.All()) != 2 {
		t.Errorf("Sweep evicted unfinished jobs: %d", len(js.All()))
	}
}

type stubCmd struct {
	stdoutR *io.PipeReader
	stdoutW *io.PipeWriter
	stderrR *io.PipeReader
	stderrW *io.PipeWriter
	done    chan struct{}
}

func newStubCmd(stdoutPayload, stderrPayload string) *stubCmd {
	r1, w1 := io.Pipe()
	r2, w2 := io.Pipe()
	c := &stubCmd{
		stdoutR: r1, stdoutW: w1,
		stderrR: r2, stderrW: w2,
		done: make(chan struct{}),
	}
	go func() {
		defer close(c.done)
		_, _ = io.WriteString(w1, stdoutPayload)
		_ = w1.Close()
		_, _ = io.WriteString(w2, stderrPayload)
		_ = w2.Close()
	}()
	return c
}

func (s *stubCmd) StdoutPipe() (io.ReadCloser, error) { return s.stdoutR, nil }
func (s *stubCmd) StderrPipe() (io.ReadCloser, error) { return s.stderrR, nil }
func (s *stubCmd) Start() error                       { return nil }
func (s *stubCmd) Wait() error                        { <-s.done; return nil }

func TestCloneJobRunWithStub(t *testing.T) {
	dir := t.TempDir()
	reg := NewRegistry(dir)
	js := NewCloneJobs()
	job := js.NewJob(CloneRequest{
		URL: "https://example.com/foo.git", ParentPath: dir, FolderName: "foo",
	})
	prev := execCommandContext
	execCommandContext = func(_ context.Context, _ string, _ ...string) cmdIface {
		return newStubCmd(
			"Cloning into 'foo'...\n",
			"Receiving objects:  42% (42/100)\n"+
				"Receiving objects: 100% (100/100), 12.34 KiB\n"+
				"Resolving deltas: 100% (10/10)\n",
		)
	}
	t.Cleanup(func() { execCommandContext = prev })

	go job.Run(context.Background(), "git", reg, nil, nil)

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		job.mu.Lock()
		state := job.State
		job.mu.Unlock()
		if state != "running" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	job.mu.Lock()
	defer job.mu.Unlock()
	if job.State != "complete" {
		t.Fatalf("job.State = %q, want complete (err=%q)", job.State, job.Error)
	}
	var sawProgress, sawLog, sawComplete bool
	for _, ev := range job.evBuf {
		switch ev.Event {
		case "progress":
			sawProgress = true
		case "log":
			sawLog = true
		case "complete":
			sawComplete = true
		}
	}
	if !sawProgress {
		t.Error("no progress event emitted")
	}
	if !sawLog {
		t.Error("no log event emitted")
	}
	if !sawComplete {
		t.Error("no complete event emitted")
	}
	got, ok := reg.Get(joinPath(dir, "foo"))
	if !ok {
		t.Errorf("project not registered: got %+v", got)
	}
}

var _ = strings.HasPrefix
