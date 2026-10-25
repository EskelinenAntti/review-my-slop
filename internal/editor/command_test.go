package editor

import (
	"context"
	"os/exec"
	"strings"
	"testing"
)

func TestShellCommandPreservesArgumentBoundaries(t *testing.T) {
	arguments := []string{
		"",
		"plain",
		"two words",
		"it's quoted",
		`"double quotes"`,
		`$HOME $(printf expanded) ` + "`printf expanded`",
		"first\nsecond",
		`back\slash`,
		"*.go",
		"; printf injected #",
	}
	command := shellCommand(
		context.Background(), "printf '%s\\000'", arguments...,
	)
	assertCommandOutput(t, command, strings.Join(arguments, "\x00")+"\x00")
}

func TestEditorCommandsTreatPathsAsLiteralArguments(t *testing.T) {
	for _, path := range []string{
		"/repo with spaces/it's.go",
		`/repo/$(printf expanded).go`,
		"/repo/file; printf injected #.go",
		"/repo/first\nsecond.go",
	} {
		t.Run(path, func(t *testing.T) {
			ctx := context.Background()
			const editor = "printf '%s\\000'"
			assertCommandOutput(
				t, shellCommand(ctx, editor, path), path+"\x00",
			)
			assertCommandOutput(
				t, sourceCommand(ctx, editor, path, 42), "+42\x00"+path+"\x00",
			)
		})
	}
}

func assertCommandOutput(t *testing.T, command *exec.Cmd, want string) {
	t.Helper()
	output, err := command.Output()
	if err != nil {
		t.Fatal(err)
	}
	if string(output) != want {
		t.Fatalf("output = %q, want %q", output, want)
	}
}
