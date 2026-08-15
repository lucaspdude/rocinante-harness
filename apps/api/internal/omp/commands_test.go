package omp

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBuildPromptFrameV2(t *testing.T) {
	frame, err := BuildPromptFrame(PromptRequest{
		Text:          "hello",
		ModelRole:     "default",
		ThinkingLevel: "low",
	}, "req_abc")
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal(frame, &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out["type"] != "prompt" {
		t.Errorf("type = %v", out["type"])
	}
	if out["text"] != "hello" {
		t.Errorf("text = %v", out["text"])
	}
	if out["client_request_id"] != "req_abc" {
		t.Errorf("client_request_id = %v", out["client_request_id"])
	}
	if out["model_role"] != "default" {
		t.Errorf("model_role = %v", out["model_role"])
	}
	if out["thinking_level"] != "low" {
		t.Errorf("thinking_level = %v", out["thinking_level"])
	}
}

func TestBuildPromptFrameEmptyText(t *testing.T) {
	_, err := BuildPromptFrame(PromptRequest{Text: "  "}, "req_x")
	if err == nil {
		t.Errorf("expected error for empty text")
	}
	if !strings.Contains(err.Error(), "text required") {
		t.Errorf("err = %v", err)
	}
}

func TestBuildAbortFrame(t *testing.T) {
	frame := BuildAbortFrame("req_abc")
	var out map[string]any
	if err := json.Unmarshal(frame, &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out["type"] != "abort" {
		t.Errorf("type = %v", out["type"])
	}
	if out["client_request_id"] != "req_abc" {
		t.Errorf("client_request_id = %v", out["client_request_id"])
	}
}

func TestBuildForkFrame(t *testing.T) {
	frame, err := BuildForkFrame(ForkRequest{AtMessageID: "msg_01JGA"}, "req_abc")
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal(frame, &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out["type"] != "fork" {
		t.Errorf("type = %v", out["type"])
	}
	if out["at_message_id"] != "msg_01JGA" {
		t.Errorf("at_message_id = %v", out["at_message_id"])
	}
}

func TestBuildForkFrameMissingID(t *testing.T) {
	_, err := BuildForkFrame(ForkRequest{}, "req_x")
	if err == nil {
		t.Errorf("expected error for missing at_message_id")
	}
}
