package editor

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

func configuredEditorCommand() (string, error) {
	command := strings.TrimSpace(os.Getenv("EDITOR"))
	if command == "" {
		return "", errors.New("$EDITOR is not set")
	}
	return command, nil
}

func newCommentEditorCommand(
	ctx context.Context,
	editor, path string,
) *exec.Cmd {
	command := editor + " " + quoteShellArgument(path)
	return exec.CommandContext(ctx, "sh", "-c", command)
}

func newSourceEditorCommand(
	ctx context.Context,
	editor, path string,
	line int,
) *exec.Cmd {
	return exec.CommandContext(
		ctx,
		"sh",
		"-c",
		editor+" +"+strconv.Itoa(line)+" "+quoteShellArgument(path),
	)
}

func quoteShellArgument(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
