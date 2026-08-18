package api

// Login endpoints — PR-01 wire format. Five routes under
// /api/v1/login/* are public (no auth): providers, start, stream
// (SSE), ack (POST), status.
//
// F4 (post-review): the login flow spawns `omp --mode rpc-ui`
// (not the legacy `omp login <id>` CLI subcommand), sends
// `{"type":"login","providerId":"X"}` on stdin, and reads JSONL
// stdout frames. ui_request frames become SSE events;
// response/login_response/auth_complete frames close the job.

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
	"time"

	"github.com/go-chi/chi/v5"
)

// LoginHandlers groups the deps for the login routes.
type LoginHandlers struct {
	Providers  LoginProvidersProvider
	Jobs       *LoginJobs
	Files      *LoginHandlersFiles
	OMPPath    string
	Logger     Logger
	CmdFactory func(ctx context.Context, name string, args []string) CmdIface
}

// LoginHandlersFiles is a placeholder for project-context integration.
type LoginHandlersFiles struct{}

// Logger is the minimal interface the login handlers need.
type Logger interface {
	Info(msg string, kv ...any)
	Error(msg string, kv ...any)
}

type nullLogger struct{}

func (nullLogger) Info(string, ...any)  {}
func (nullLogger) Error(string, ...any) {}

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
		// F4 ack roundtrip: forward to the omp child via the
		// job's stdin.
		if w := job.Stdin(); w != nil {
			frame := fmt.Sprintf(
				`{"type":"extension_ui_response","id":%q,"value":%q}`+"\n",
				ack.Value, ack.Value,
			)
			_, _ = io.WriteString(w, frame)
		}
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
// no-op "ui_request" otherwise.
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

	// Spawn `omp --mode rpc-ui` and send the JSONL login frame
	// (F4 spec). The harness reads stdout line-by-line.
	job.publish(LoginEvent{
		Event: "spawn",
		Data: map[string]any{
			"omp_bin": h.OMPPath,
			"argv":    []string{"--mode", "rpc-ui"},
		},
	})
	cmd := h.CmdFactory(ctx, h.OMPPath, []string{"--mode", "rpc-ui"})
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("pipe: %w", err)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}
	job.SetStdin(stdin)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("spawn: %w", err)
	}
	loginFrame := fmt.Sprintf(`{"type":"login","providerId":%q}`+"\n", job.ProviderID)
	if _, err := io.WriteString(stdin, loginFrame); err != nil {
		_ = stdin.Close()
		return fmt.Errorf("write login frame: %w", err)
	}

	scanErrCh := make(chan error, 1)
	go func() {
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}
			var payload map[string]any
			if uerr := json.Unmarshal(line, &payload); uerr != nil {
				job.publish(LoginEvent{
					Event: "log",
					Data:  map[string]any{"line": string(line)},
				})
				continue
			}
			frameType, _ := payload["type"].(string)
			switch frameType {
			case "extension_ui_request":
				job.publish(LoginEvent{
					Event: "ui_request",
					Data:  payload,
				})
			case "response", "login_response", "login_complete", "auth_complete":
				job.publish(LoginEvent{
					Event: "auth_complete",
					Data:  payload,
				})
			default:
				job.publish(LoginEvent{
					Event: "log",
					Data:  map[string]any{"line": string(line)},
				})
			}
		}
		scanErrCh <- scanner.Err()
	}()
	waitErr := cmd.Wait()
	<-scanErrCh
	_ = stdin.Close()
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

	past, unsub, _ := job.Subscribe()
	defer unsub()
	for _, ev := range past {
		if !writeSSE(w, ev.Event, ev.Data) {
			return
		}
	}
	flusher.Flush()

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
		cur := h.Jobs.Get(jobID)
		if cur == nil {
			return
		}
		snap := cur.Snapshot()
		if snap.State != "running" {
			_ = writeSSE(w, "status", map[string]any{
				"job_id": snap.JobID,
				"state":  snap.State,
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

// silence unused import
var _ = exec.CommandContext
