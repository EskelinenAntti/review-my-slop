package editor

import (
	"context"
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
