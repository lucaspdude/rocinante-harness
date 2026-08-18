package api

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"testing"

	"github.com/lucaspdude/rocinante-harness/apps/api/internal/catalog"
)

// nopCloser wraps an io.Reader as io.ReadCloser (no-op close).
type nopCloser struct{ io.Reader }

func (nopCloser) Close() error { return nil }

// nopWriteCloser wraps an io.Writer as io.WriteCloser.
type nopWriteCloser struct{ io.Writer }

func (nopWriteCloser) Close() error { return nil }

type bufferCmd struct {
	stdoutBuf *bytes.Buffer
	stdinBuf  *bytes.Buffer
}

func newBufferCmd(stdoutPayload string) *bufferCmd {
	return &bufferCmd{
		stdoutBuf: bytes.NewBufferString(stdoutPayload),
		stdinBuf:  &bytes.Buffer{},
	}
}

func (c *bufferCmd) StdinPipe() (io.WriteCloser, error)  { return nopWriteCloser{c.stdinBuf}, nil }
func (c *bufferCmd) StdoutPipe() (io.ReadCloser, error) { return nopCloser{c.stdoutBuf}, nil }
func (c *bufferCmd) StderrPipe() (io.ReadCloser, error) {
	return nopCloser{bytes.NewReader(nil)}, nil
}
func (c *bufferCmd) Start() error { return nil }
func (c *bufferCmd) Wait() error  { return nil }

type stubProbe map[string]bool

func (s stubProbe) IsConfigured(name string) bool { return s[name] }

func withExecDynamic(t *testing.T, factory func(context.Context, string, ...string) cmdIface, fn func()) {
	t.Helper()
	prev := execCommandContextDynamic
	execCommandContextDynamic = factory
	defer func() { execCommandContextDynamic = prev }()
	fn()
}

func TestDynamicProviderFallsBackOnEmptyBin(t *testing.T) {
	probe := stubProbe{}
	fallback := NewStaticLoginProviders(probe)
	dyn := NewOMPLoginProviders("", probe, fallback)
	got := dyn.List()
	if len(got) != 5 {
		t.Errorf("expected 5 fallback providers, got %d", len(got))
	}
}

func TestDynamicProviderFallsBackOnSpawnFail(t *testing.T) {
	probe := stubProbe{"anthropic": true}
	fallback := NewStaticLoginProviders(probe)
	dyn := NewOMPLoginProviders("/nonexistent/binary", probe, fallback)

	withExecDynamic(t, func(_ context.Context, _ string, args ...string) cmdIface {
		return failingCmd{}
	}, func() {
		got := dyn.List()
		if len(got) == 0 {
			t.Fatal("expected fallback to populate list")
		}
	})
}

type failingCmd struct{}

func (failingCmd) StdinPipe() (io.WriteCloser, error)  { return nil, fmt.Errorf("fail") }
func (failingCmd) StdoutPipe() (io.ReadCloser, error) { return nil, fmt.Errorf("fail") }
func (failingCmd) StderrPipe() (io.ReadCloser, error)  { return nil, fmt.Errorf("fail") }
func (failingCmd) Start() error                        { return fmt.Errorf("fail") }
func (failingCmd) Wait() error                         { return fmt.Errorf("fail") }

func TestDynamicProviderParsesOmpTopLevelList(t *testing.T) {
	probe := stubProbe{"anthropic": true}
	fallback := NewStaticLoginProviders(probe)
	dyn := NewOMPLoginProviders("/bin/true", probe, fallback)

	payload := `{"type":"response","list":[{"id":"anthropic","name":"Anthropic","available":true,"authenticated":true,"env_var":"ANTHROPIC_API_KEY"},{"id":"openai","name":"OpenAI","available":true,"authenticated":false,"env_var":"OPENAI_API_KEY"},{"id":"kimi-code","name":"Kimi Code","available":true,"authenticated":true}]}` + "\n"

	withExecDynamic(t, func(_ context.Context, _ string, args ...string) cmdIface {
		return newBufferCmd(payload)
	}, func() {
		got := dyn.List()
		if len(got) != 3 {
			t.Fatalf("len = %d, want 3 (got: %+v)", len(got), got)
		}
		var kimi *catalog.LoginProviderInfo
		for i, p := range got {
			if p.ID == "kimi-code" {
				kimi = &got[i]
			}
		}
		if kimi == nil {
			t.Fatal("kimi-code not in list")
		}
		if !kimi.Authenticated {
			t.Error("kimi-code not authenticated per omp")
		}
		if !kimi.Keyless {
			t.Errorf("kimi-code should be keyless (no env var); got %v", kimi.Keyless)
		}
		if len(kimi.EnvVars) != 0 {
			t.Errorf("kimi-code should have empty env_vars; got %v", kimi.EnvVars)
		}
	})
}

func TestDynamicProviderCrossRefsProbeForAuth(t *testing.T) {
	probe := stubProbe{"openai": true}
	fallback := NewStaticLoginProviders(probe)
	dyn := NewOMPLoginProviders("/bin/true", probe, fallback)

	payload := `{"type":"response","list":[{"id":"openai","name":"OpenAI","available":true,"authenticated":false,"env_var":"OPENAI_API_KEY"}]}` + "\n"

	withExecDynamic(t, func(_ context.Context, _ string, args ...string) cmdIface {
		return newBufferCmd(payload)
	}, func() {
		got := dyn.List()
		var openai *catalog.LoginProviderInfo
		for i, p := range got {
			if p.ID == "openai" {
				openai = &got[i]
			}
		}
		if openai == nil {
			t.Fatal("openai not in list")
		}
		if !openai.Authenticated {
			t.Error("openai should be authenticated per probe (key on disk)")
		}
	})
}

func TestDynamicProviderHandlesResultNestedList(t *testing.T) {
	probe := stubProbe{}
	fallback := NewStaticLoginProviders(probe)
	dyn := NewOMPLoginProviders("/bin/true", probe, fallback)

	payload := `{"type":"response","result":{"list":[{"id":"anthropic","name":"Anthropic","available":true,"authenticated":true,"env_var":"ANTHROPIC_API_KEY"}]}}` + "\n"

	withExecDynamic(t, func(_ context.Context, _ string, args ...string) cmdIface {
		return newBufferCmd(payload)
	}, func() {
		got := dyn.List()
		if len(got) != 1 {
			t.Fatalf("len = %d, want 1", len(got))
		}
		if got[0].ID != "anthropic" {
			t.Errorf("ID = %q", got[0].ID)
		}
	})
}

func TestDynamicProviderHandlesMalformedJSON(t *testing.T) {
	probe := stubProbe{}
	fallback := NewStaticLoginProviders(probe)
	dyn := NewOMPLoginProviders("/bin/true", probe, fallback)

	payload := ""
	for i := 0; i < 20; i++ {
		payload += "not json\n"
	}
	payload += "\n"

	withExecDynamic(t, func(_ context.Context, _ string, args ...string) cmdIface {
		return newBufferCmd(payload)
	}, func() {
		got := dyn.List()
		if len(got) == 0 {
			t.Fatal("expected fallback to populate")
		}
	})
}

func TestDynamicProviderHandlesErrorResponse(t *testing.T) {
	probe := stubProbe{}
	fallback := NewStaticLoginProviders(probe)
	dyn := NewOMPLoginProviders("/bin/true", probe, fallback)

	payload := `{"type":"response","error":"not supported"}` + "\n"

	withExecDynamic(t, func(_ context.Context, _ string, args ...string) cmdIface {
		return newBufferCmd(payload)
	}, func() {
		got := dyn.List()
		if len(got) == 0 {
			t.Fatal("expected fallback to populate")
		}
	})
}
