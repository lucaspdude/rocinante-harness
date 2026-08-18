package api

import (
	"context"
	"io"
	"os/exec"
)

// cmdIface is the subset of *exec.Cmd the login flow needs.
// Tests can swap in a fake impl that pipes canned output.
type cmdIface interface {
	StdinPipe() (io.WriteCloser, error)
	StdoutPipe() (io.ReadCloser, error)
	StderrPipe() (io.ReadCloser, error)
	Start() error
	Wait() error
}

// defaultCmdFactory wraps exec.CommandContext into a cmdIface. Used
// by both the login flow (PR-01) and the dynamic provider
// discovery (F1).
func defaultCmdFactory(ctx context.Context, name string, args ...string) cmdIface {
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
