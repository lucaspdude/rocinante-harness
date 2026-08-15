package omp

import (
	"encoding/json"
	"fmt"
	"strings"
)

// PromptRequest is the NDJSON payload sent to omp for a prompt.
type PromptRequest struct {
	Text          string `json:"text"`
	ModelRole     string `json:"model_role,omitempty"`
	ThinkingLevel string `json:"thinking_level,omitempty"`
	Model         string `json:"model,omitempty"`
}

// BuildPromptFrame produces the JSON request body for a prompt
// command, matching the omp RPC v1 + v2 envelope.
func BuildPromptFrame(req PromptRequest, clientRequestID string) ([]byte, error) {
	if strings.TrimSpace(req.Text) == "" {
		return nil, fmt.Errorf("prompt text required")
	}
	out := map[string]any{
		"type":              "prompt",
		"text":              req.Text,
		"client_request_id": clientRequestID,
	}
	if req.ModelRole != "" {
		out["model_role"] = req.ModelRole
	}
	if req.ThinkingLevel != "" {
		out["thinking_level"] = req.ThinkingLevel
	}
	if req.Model != "" {
		out["model"] = req.Model
	}
	return json.Marshal(out)
}

// BuildAbortFrame produces the request body for an abort command.
func BuildAbortFrame(clientRequestID string) []byte {
	out := map[string]any{
		"type":              "abort",
		"client_request_id": clientRequestID,
	}
	b, _ := json.Marshal(out)
	return b
}

// ForkRequest is the body of a fork command.
type ForkRequest struct {
	AtMessageID string `json:"at_message_id"`
}

// BuildForkFrame produces the request body for a fork command.
func BuildForkFrame(req ForkRequest, clientRequestID string) ([]byte, error) {
	if strings.TrimSpace(req.AtMessageID) == "" {
		return nil, fmt.Errorf("at_message_id required")
	}
	out := map[string]any{
		"type":              "fork",
		"at_message_id":     req.AtMessageID,
		"client_request_id": clientRequestID,
	}
	return json.Marshal(out)
}

// SendCommand writes a frame to the session's stdin. The caller is
// responsible for serializing against concurrent writers.
func (s *Session) SendCommand(frame []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return fmt.Errorf("session closed")
	}
	if _, err := s.stdin.Write(frame); err != nil {
		return fmt.Errorf("write: %w", err)
	}
	if _, err := s.stdin.Write([]byte("\n")); err != nil {
		return fmt.Errorf("write newline: %w", err)
	}
	return nil
}
