package api

// Login endpoints — PR-01 wire format. Five routes under
// /api/v1/login/* are public (no auth): providers, start, stream
// (SSE), ack (POST), status.
//
// The flow:
//   1. GET /api/v1/login/providers      → list of available providers
//   2. POST /api/v1/login/start/{id}    → 202 with job_id
//   3. GET /api/v1/login/{id}/stream    → SSE event stream
//   4. POST /api/v1/login/{id}/ack      → user response
//   5. GET /api/v1/login/{id}/status    → terminal state
//
// For the MVP we don't shell out to a real omp child — the
// keystore write path remains the canonical way to set a key, and
// the /login lifecycle exists so the UI has a stable shape across
// all 5 known providers. PR-01 follow-up can wire the omp child.

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
)

// LoginHandlers groups the dependencies needed by the login routes.
type LoginHandlers struct {
	Providers *LoginProvidersCache
	Jobs      *LoginJobs
	OMPPath   string // optional — path to the omp binary; empty disables the spawn
	Logger    Logger // optional structured logger
	// CmdFactory allows tests to stub exec.CommandContext; production wires osExec.
	CmdFactory func(ctx context.Context, name string, args []string) CmdIface
}

// Logger is the minimal interface the login handlers need.
type Logger interface {
	Info(msg string, kv ...any)
	Error(msg string, kv ...any)
}
// OsExec is the production CmdFactory; it shells out via os/exec.
func OsExec(ctx context.Context, name string, args []string) CmdIface {
	return &realCmd{Cmd: exec.CommandContext(ctx, name, args...)}
}

type nullLogger struct{}

func (nullLogger) Info(string, ...any) {}
func (nullLogger) Error(string, ...any) {}

// CmdIface is the subset of *exec.Cmd the login flow needs. Tests
// can swap in a fake impl that pipes canned output.
type CmdIface interface {
	StdoutPipe() (io.ReadCloser, error)
	Start() error
	Wait() error
}


type realCmd struct{ Cmd *exec.Cmd }

func (r *realCmd) StdoutPipe() (io.ReadCloser, error) { return r.Cmd.StdoutPipe() }
func (r *realCmd) Start() error                       { return r.Cmd.Start() }
func (r *realCmd) Wait() error                        { return r.Cmd.Wait() }

// LoginProvidersHandler serves GET /api/v1/login/providers.
func (h *LoginHandlers) LoginProvidersHandler(w http.ResponseWriter, r *http.Request) {
	if h.Providers == nil {
		writeJSON(w, http.StatusOK, LoginProvidersResponse{
			Providers: []LoginProviderInfo{},
			CachedAt:  time.Now().UTC(),
		})
		return
	}
	items := h.Providers.Snapshot()
	writeJSON(w, http.StatusOK, LoginProvidersResponse{
		Providers: items,
		CachedAt:  time.Now().UTC(),
	})
}

// LoginStartHandler is POST /api/v1/login/start/{provider}. It
// returns 202 with a job_id; the client subscribes via SSE.
func (h *LoginHandlers) LoginStartHandler(w http.ResponseWriter, r *http.Request) {
	providerID := chi.URLParam(r, "provider")
	if providerID == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{
			Code:    "bad_request",
			Message: "provider id is required",
		})
		return
	}

	ctx, cancel := context.WithCancel(r.Context())
	job := h.Jobs.NewJob(providerID, cancel)
	job.SetResponder(func(ack LoginAck) error {
		job.publish(LoginEvent{
			Event: "ack",
			Data: map[string]any{
				"received": true,
				"length":   len(ack.Value),
			},
		})
		return nil
	})

	go h.runLoginJob(ctx, job)

	writeJSON(w, http.StatusAccepted, LoginStartResponse{
		JobID:      job.ID,
		StreamURL:  fmt.Sprintf("/api/v1/login/%s/stream", job.ID),
		StatusURL:  fmt.Sprintf("/api/v1/login/%s/status", job.ID),
		ProviderID: providerID,
	})
}

// runLoginJob drives the omp child (when configured) or a
// no-op "ui_request" otherwise. Either way, the job closes when
// stdout is exhausted.
func (h *LoginHandlers) runLoginJob(ctx context.Context, job *LoginJob) {
	defer job.Finish(LoginJobComplete, "")

	err := h.runJob(ctx, job)
	if err != nil && !errors.Is(err, context.Canceled) {
		h.log().Info("login job exited with error", "job_id", job.ID, "err", err)
		job.Finish(LoginJobFailed, err.Error())
		return
	}
}

func (h *LoginHandlers) runJob(ctx context.Context, job *LoginJob) error {
	if h.OMPPath == "" || h.CmdFactory == nil {
		// Static catalog: emit a single ack prompt and complete.
		job.publish(LoginEvent{
			Event: "ui_request",
			Data: map[string]any{
				"method": "notice",
				"title":  fmt.Sprintf("Use Settings → Providers to set the %s key.", job.ProviderID),
				"detail": "Paste-key flow runs on the existing /providers endpoint; this /login job is the lifecycle seam.",
			},
		})
		select {
		case <-time.After(1500 * time.Millisecond):
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	job.publish(LoginEvent{
		Event: "spawn",
		Data: map[string]any{
			"omp_bin": h.OMPPath,
			"argv":    []string{"login", job.ProviderID},
		},
	})
	cmd := h.CmdFactory(ctx, h.OMPPath, []string{"login", job.ProviderID})
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("spawn: %w", err)
	}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}
			var payload map[string]any
			if uerr := json.Unmarshal(line, &payload); uerr == nil {
				if method, _ := payload["method"].(string); method != "" {
					job.publish(LoginEvent{
						Event: "ui_request",
						Data:  payload,
					})
					continue
				}
			}
			job.publish(LoginEvent{
				Event: "log",
				Data:  map[string]any{"line": string(line)},
			})
		}
	}()
	waitErr := cmd.Wait()
	wg.Wait()
	return waitErr
}

// LoginStreamHandler is GET /api/v1/login/{jobId}/stream. SSE with
// 15s heartbeats.
func (h *LoginHandlers) LoginStreamHandler(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "jobId")
	job := h.Jobs.Get(jobID)
	if job == nil {
		writeJSON(w, http.StatusNotFound, errorResponse{Code: "job_not_found"})
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Code: "no_flusher"})
		return
	}
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	past, unsub, err := job.Subscribe()
	if err != nil {
		_ = writeSSE(w, "status", map[string]any{
			"job_id": job.ID,
			"state":  string(job.State()),
		})
		flusher.Flush()
		return
	}
	defer unsub()

	for _, ev := range past {
		if !writeSSE(w, ev.Event, ev.Data) {
			return
		}
	}
	flusher.Flush()

	// 15s heartbeat ticker. While the ticker hasn't fired, poll
	// the job's terminal state every 50ms so we emit a final
	// "status" event and close promptly.
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			if _, err := fmt.Fprint(w, ":keepalive\n\n"); err != nil {
				return
			}
			flusher.Flush()
			continue
		default:
		}
		if job.State() != LoginJobRunning {
			_ = writeSSE(w, "status", map[string]any{
				"job_id": job.ID,
				"state":  string(job.State()),
			})
			flusher.Flush()
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// LoginAckHandler is POST /api/v1/login/{jobId}/ack.
func (h *LoginHandlers) LoginAckHandler(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "jobId")
	job := h.Jobs.Get(jobID)
	if job == nil {
		writeJSON(w, http.StatusNotFound, errorResponse{Code: "job_not_found"})
		return
	}
	if job.State() != LoginJobRunning {
		writeJSON(w, http.StatusGone, errorResponse{Code: "job_expired"})
		return
	}
	var ack LoginAck
	if err := json.NewDecoder(r.Body).Decode(&ack); err != nil {
		// Empty body is allowed — it's a cancel signal.
		if !errors.Is(err, io.EOF) {
			writeJSON(w, http.StatusBadRequest, errorResponse{
				Code:    "bad_request",
				Message: err.Error(),
			})
			return
		}
	}
	if err := job.Respond(ack); err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{
			Code:    "ack_failed",
			Message: err.Error(),
		})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// LoginStatusHandler is GET /api/v1/login/{jobId}/status.
func (h *LoginHandlers) LoginStatusHandler(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "jobId")
	job := h.Jobs.Get(jobID)
	if job == nil {
		writeJSON(w, http.StatusNotFound, errorResponse{Code: "job_not_found"})
		return
	}
	writeJSON(w, http.StatusOK, job.Snapshot())
}

func (h *LoginHandlers) log() Logger {
	if h.Logger == nil {
		return nullLogger{}
	}
	return h.Logger
}

// writeSSE writes a single SSE event frame.
func writeSSE(w http.ResponseWriter, event string, data map[string]any) bool {
	if event != "" {
		if _, err := fmt.Fprintf(w, "event: %s\n", sanitizeEventName(event)); err != nil {
			return false
		}
	}
	body, err := json.Marshal(data)
	if err != nil {
		return false
	}
	if _, err := fmt.Fprintf(w, "data: %s\n\n", strings.ReplaceAll(string(body), "\n", " ")); err != nil {
		return false
	}
	return true
}

func sanitizeEventName(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '_' || r == '-':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	return b.String()
}
