package editor

import (
	"context"
	"io"
	"os/exec"
	"strconv"
	"strings"
)

func sourceCommand(
	ctx context.Context,
	editor, path string,
	line int,
) *exec.Cmd {
	return shellCommand(ctx, editor, "+"+strconv.Itoa(line), path)
}

func shellCommand(
	ctx context.Context,
	editor string,
	arguments ...string,
) *exec.Cmd {
	quoted := make([]string, len(arguments))
	for index, argument := range arguments {
		quoted[index] = shellQuote(argument)
	}
	command := editor + " " + strings.Join(quoted, " ")
	return exec.CommandContext(ctx, "sh", "-c", command)
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
