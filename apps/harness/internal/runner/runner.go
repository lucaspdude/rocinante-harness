package runner

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

// Process tracks a single child process.
type Process struct {
	Cmd  *exec.Cmd
	Name string
	PID  int
	Log  *os.File
}

// State is the on-disk representation of the running harness.
type State struct {
	APIPID    int       `json:"api_pid"`
	WebPID    int       `json:"web_pid"`
	APIPort   int       `json:"api_port"`
	WebPort   int       `json:"web_port"`
	StartedAt time.Time `json:"started_at"`
}

// Runner holds the active subprocesses and the state file path.
type Runner struct {
	mu       sync.Mutex
	procs    map[string]*Process
	state    State
	cacheDir string
	logDir   string
}

// New returns a Runner anchored at the given cache dir.
func New(cacheDir, logDir string) *Runner {
	return &Runner{
		procs:    make(map[string]*Process),
		cacheDir: cacheDir,
		logDir:   logDir,
	}
}

// StartAPI starts the api binary.
func (r *Runner) StartAPI(ctx context.Context, apiBin string, port int, shareDir string) error {
	if err := os.MkdirAll(r.logDir, 0o700); err != nil {
		return err
	}
	logFile, err := os.OpenFile(filepath.Join(r.logDir, "api.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, apiBin, "--port", fmt.Sprintf("%d", port), "--share-dir", shareDir)
	cmd.Env = append(os.Environ(),
		"ROCHASSEN_SHARE_DIR="+shareDir,
	)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		return err
	}
	r.mu.Lock()
	r.procs["api"] = &Process{Cmd: cmd, Name: "api", PID: cmd.Process.Pid, Log: logFile}
	r.state.APIPID = cmd.Process.Pid
	r.state.APIPort = port
	r.state.StartedAt = time.Now()
	r.mu.Unlock()
	return nil
}

// StartWeb starts the Next standalone server. The webDir must
// point to a directory produced by `next build` with
// `output: 'standalone'`. The launcher looks for server.js at
// both the conventional location
// (<webDir>/apps/web/server.js) and the simpler fallback
// (<webDir>/server.js).
func (r *Runner) StartWeb(ctx context.Context, webDir string, port int) error {
	if err := os.MkdirAll(r.logDir, 0o700); err != nil {
		return err
	}
	logFile, err := os.OpenFile(filepath.Join(r.logDir, "web.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}

	candidates := []string{
		filepath.Join(webDir, "apps", "web", "server.js"),
		filepath.Join(webDir, "server.js"),
	}
	var serverPath string
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			serverPath = c
			break
		}
	}
	if serverPath == "" {
		_ = logFile.Close()
		return fmt.Errorf("standalone server.js not found under %s (tried %v)", webDir, candidates)
	}

	cmd := exec.CommandContext(ctx, "node", serverPath)
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("PORT=%d", port),
		"HOSTNAME=0.0.0.0",
	)
	if apiUrl := os.Getenv("ROCINANTE_API_INTERNAL_URL"); apiUrl != "" {
		cmd.Env = append(cmd.Env, "RH_API_INTERNAL_URL="+apiUrl)
	}
	if publicApi := os.Getenv("ROCINANTE_PUBLIC_API_URL"); publicApi != "" {
		cmd.Env = append(cmd.Env, "NEXT_PUBLIC_RH_API_URL="+publicApi)
	}
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		return err
	}
	r.mu.Lock()
	r.procs["web"] = &Process{Cmd: cmd, Name: "web", PID: cmd.Process.Pid, Log: logFile}
	r.state.WebPID = cmd.Process.Pid
	r.state.WebPort = port
	r.mu.Unlock()
	return nil
}

// StopAll sends SIGTERM to every running child, waits 5s, then
// SIGKILL the survivors.
func (r *Runner) StopAll() error {
	r.mu.Lock()
	procs := make([]*Process, 0, len(r.procs))
	for _, p := range r.procs {
		procs = append(procs, p)
	}
	r.mu.Unlock()

	var firstErr error
	for _, p := range procs {
		if p.Cmd == nil || p.Cmd.Process == nil {
			continue
		}
		_ = p.Cmd.Process.Signal(syscall.SIGTERM)
	}
	deadline := time.After(5 * time.Second)
	done := make(chan struct{})
	go func() {
		for _, p := range procs {
			if p.Cmd == nil {
				continue
			}
			_ = p.Cmd.Wait()
		}
		close(done)
	}()
	select {
	case <-done:
	case <-deadline:
		for _, p := range procs {
			if p.Cmd == nil || p.Cmd.Process == nil {
				continue
			}
			_ = p.Cmd.Process.Kill()
		}
	}
	for _, p := range procs {
		if p.Log != nil {
			_ = p.Log.Close()
		}
	}
	return firstErr
}

// State returns the current recorded state.
func (r *Runner) State() State {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.state
}

// PIDFile writes the state to ${CACHE_DIR}/roc-harness/default.json.
func (r *Runner) PIDFile() error {
	if err := os.MkdirAll(r.cacheDir, 0o700); err != nil {
		return err
	}
	body, err := jsonMarshalIndent(r.State())
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(r.cacheDir, "default.json"), body, 0o600)
}

// PidFileExists returns true if the harness has a recorded state.
func PidFileExists(cacheDir string) bool {
	_, err := os.Stat(filepath.Join(cacheDir, "default.json"))
	return err == nil
}

// LoadState reads the harness state from disk.
func LoadState(cacheDir string) (State, error) {
	body, err := os.ReadFile(filepath.Join(cacheDir, "default.json"))
	if err != nil {
		return State{}, err
	}
	var s State
	if err := jsonUnmarshal(body, &s); err != nil {
		return State{}, err
	}
	return s, nil
}

// TailLogs opens the log files and tails the last N lines.
func TailLogs(logDir string, n int) (string, error) {
	api, err := tailFile(filepath.Join(logDir, "api.log"), n)
	if err != nil && !errorsIs(err, os.ErrNotExist) {
		return "", err
	}
	web, err := tailFile(filepath.Join(logDir, "web.log"), n)
	if err != nil && !errorsIs(err, os.ErrNotExist) {
		return "", err
	}
	return "=== api.log ===\n" + api + "\n=== web.log ===\n" + web, nil
}

func tailFile(path string, n int) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	lines := make([]string, 0, n)
	scanner := bufioNewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
		if len(lines) > n {
			lines = lines[1:]
		}
	}
	out := ""
	for _, l := range lines {
		out += l + "\n"
	}
	return out, nil
}

// PipeError writes any unread error to the underlying writer.
func PipeError(r io.Reader, w io.Writer) {
	_, _ = io.Copy(w, r)
}
