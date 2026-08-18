package api

import (
	"context"
	"io"
	"os/exec"
)

// CmdIface is the subset of *exec.Cmd the login flow needs.
// Exported so callers in other packages (e.g. cmd/api/main) can
// use the same seam.
type CmdIface interface {
	StdinPipe() (io.WriteCloser, error)
	StdoutPipe() (io.ReadCloser, error)
	StderrPipe() (io.ReadCloser, error)
	Start() error
	Wait() error
}

// defaultCmdFactory wraps exec.CommandContext into a cmdIface. Used
// by both the login flow (PR-01) and the dynamic provider
// discovery (F1).
func defaultCmdFactory(ctx context.Context, name string, args ...string) CmdIface {
	return &osCmd{Cmd: exec.CommandContext(ctx, name, args...)}
}

// OSExec is the production CmdFactory exposed for main.go. Variadic
// so callers can wrap it in a non-variadic closure.
func OSExec(ctx context.Context, name string, args ...string) CmdIface {
	return &osCmd{Cmd: exec.CommandContext(ctx, name, args...)}
}

type osCmd struct {
	Cmd *exec.Cmd
}

func (c *osCmd) StdinPipe() (io.WriteCloser, error)  { return c.Cmd.StdinPipe() }
func (c *osCmd) StdoutPipe() (io.ReadCloser, error) { return c.Cmd.StdoutPipe() }
func (c *osCmd) StderrPipe() (io.ReadCloser, error)  { return c.Cmd.StderrPipe() }
func (c *osCmd) Start() error                        { return c.Cmd.Start() }
func (c *osCmd) Wait() error                         { return c.Cmd.Wait() }
