package editor

import (
	"context"
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

// shellCommand preserves the editor's shell syntax while treating arguments as
// literal values.
func shellCommand(
	ctx context.Context,
	editor string,
	arguments ...string,
) *exec.Cmd {
	quoted := make([]string, len(arguments))
	for index, argument := range arguments {
		quoted[index] = shellQuote(argument)
	}
	return exec.CommandContext(
		ctx, "sh", "-c", editor+" "+strings.Join(quoted, " "),
	)
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
