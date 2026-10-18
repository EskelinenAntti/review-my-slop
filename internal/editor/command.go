package editor

import (
	"context"
	"io"
	"os/exec"
	"strconv"
	"strings"
)

func commentCommand(ctx context.Context, editor, path string) *exec.Cmd {
	return shellCommand(ctx, editor, shellQuote(path))
}

func sourceCommand(
	ctx context.Context,
	editor, path string,
	line int,
) *exec.Cmd {
	arguments := "+" + strconv.Itoa(line) + " " + shellQuote(path)
	return shellCommand(ctx, editor, arguments)
}

func shellCommand(ctx context.Context, editor, arguments string) *exec.Cmd {
	return exec.CommandContext(ctx, "sh", "-c", editor+" "+arguments)
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

type commandAdapter struct {
	run            func() error
	stdin          io.Reader
	stdout, stderr io.Writer
}

func (c *commandAdapter) Run() error            { return c.run() }
func (c *commandAdapter) SetStdin(r io.Reader)  { c.stdin = r }
func (c *commandAdapter) SetStdout(w io.Writer) { c.stdout = w }
func (c *commandAdapter) SetStderr(w io.Writer) { c.stderr = w }
func (c *commandAdapter) execute(cmd *exec.Cmd) error {
	cmd.Stdin, cmd.Stdout, cmd.Stderr = c.stdin, c.stdout, c.stderr
	return cmd.Run()
}
