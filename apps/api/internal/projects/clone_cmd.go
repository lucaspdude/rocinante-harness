package projects

// Glue that wires exec.CommandContext into the cmdIface seam. Lives
// in its own file so production and tests can swap easily.

import (
	"context"
	"io"
	"os/exec"
)

func defaultCommandContext(ctx context.Context, name string, args ...string) cmdIface {
	return &osCmd{Cmd: exec.CommandContext(ctx, name, args...)}
}

type osCmd struct {
	Cmd *exec.Cmd
}

func (c *osCmd) StdoutPipe() (io.ReadCloser, error) {
	return c.Cmd.StdoutPipe()
}
func (c *osCmd) StderrPipe() (io.ReadCloser, error) {
	return c.Cmd.StderrPipe()
}
func (c *osCmd) Start() error      { return c.Cmd.Start() }
func (c *osCmd) Wait() error       { return c.Cmd.Wait() }

// silence unused stderr warning in build slices.
var _ = io.EOF
