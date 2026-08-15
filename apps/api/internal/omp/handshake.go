package omp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Handshake holds the result of the first NDJSON line from omp.
type Handshake struct {
	ProtocolVersion int
	OmpVersion      string
}

// omp returns handshake frames in two flavours: v1 uses snake_case
// `protocol_version` + `omp_version`, v2 uses camelCase
// `protocolVersion` + `ompVersion`. Both are accepted as the v1
// envelope; "ready" frames also expose the active protocolVersion.
func parseHandshake(line string) (Handshake, error) {
	line = strings.TrimSpace(line)
	if line == "" {
		return Handshake{}, errors.New("empty handshake line")
	}

	var generic map[string]json.RawMessage
	if err := json.Unmarshal([]byte(line), &generic); err != nil {
		return Handshake{}, fmt.Errorf("invalid handshake json: %w", err)
	}

	if raw, ok := generic["protocol_version"]; ok {
		return Handshake{ProtocolVersion: decodeInt(raw), OmpVersion: decodeString(generic["omp_version"])}, nil
	}
	if raw, ok := generic["protocolVersion"]; ok {
		return Handshake{ProtocolVersion: decodeInt(raw), OmpVersion: decodeString(generic["ompVersion"])}, nil
	}

	if _, ok := generic["jsonrpc"]; ok {
		return Handshake{ProtocolVersion: 1}, nil
	}

	return Handshake{}, errors.New("handshake missing protocol_version and jsonrpc")
}

func decodeInt(r json.RawMessage) int {
	var v int
	if err := json.Unmarshal(r, &v); err == nil {
		return v
	}
	return 0
}

func decodeString(r json.RawMessage) string {
	var v string
	if err := json.Unmarshal(r, &v); err == nil {
		return v
	}
	return ""
}

// readHandshake reads one line from the reader and dispatches to
// parseHandshake. Bounded by ctx.
func readHandshake(ctx context.Context, r *bufio.Reader) (Handshake, error) {
	type result struct {
		line string
		err  error
	}
	done := make(chan result, 1)
	go func() {
		line, err := r.ReadString('\n')
		done <- result{line: line, err: err}
	}()
	select {
	case <-ctx.Done():
		return Handshake{}, ctx.Err()
	case r := <-done:
		if r.err != nil {
			return Handshake{}, fmt.Errorf("read handshake: %w", r.err)
		}
		return parseHandshake(r.line)
	}
}

// fallbackOmpVersion shells out to `omp --version` when the v1
// handshake did not declare omp_version. Bounded to 500ms.
func fallbackOmpVersion(ctx context.Context, ompBin string) (string, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()

	cmd := exec.CommandContext(timeoutCtx, ompBin, "--version")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	v := strings.TrimSpace(string(out))
	if v == "" {
		return "", errors.New("empty --version output")
	}
	if !strings.HasPrefix(v, "omp/") {
		v = "omp/" + v
	}
	return v, nil
}
