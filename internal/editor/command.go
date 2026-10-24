package editor

import (
	"context"
	"os/exec"
	"strconv"
	"strings"
)

func commentCommand(ctx context.Context, editor, path string) *exec.Cmd {
	return shellCommand(ctx, editor, path)
}

func sourceCommand(
	ctx context.Context,
	editor, path string,
	line int,
) *exec.Cmd {
	return shellCommand(ctx, editor, "+"+strconv.Itoa(line), path)
}

// shellCommand preserves shell syntax in the editor setting while treating
// appended arguments as literal values.
func shellCommand(
	ctx context.Context,
	editor string,
	arguments ...string,
) *exec.Cmd {
	command := editor
	for _, argument := range arguments {
		command += " " + shellQuote(argument)
	}
	return exec.CommandContext(ctx, "sh", "-c", command)
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
