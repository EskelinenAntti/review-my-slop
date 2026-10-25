package editor

import (
	"context"
	"strings"
	"testing"
)

func TestShellCommandPreservesLiteralArguments(t *testing.T) {
	for _, argument := range []string{
		"",
		"/repo with spaces/it's.go",
		"$HOME",
		"$(printf expanded)",
		"first; printf injected",
		"line\nbreak",
		"*.go",
	} {
		t.Run(argument, func(t *testing.T) {
			command := shellCommand(
				context.Background(), "printf '<%s>'", argument,
			)
			output, err := command.Output()
			if err != nil {
				t.Fatal(err)
			}
			if got, want := string(output), "<"+argument+">"; got != want {
				t.Fatalf("output = %q, want %q", got, want)
			}
		})
	}
}

func TestShellCommandPreservesArgumentBoundaries(t *testing.T) {
	arguments := []string{"", "two words", "it's.go"}
	command := shellCommand(
		context.Background(), "printf '<%s>\\n'", arguments...,
	)
	output, err := command.Output()
	if err != nil {
		t.Fatal(err)
	}
	want := "<" + strings.Join(arguments, ">\n<") + ">\n"
	if string(output) != want {
		t.Fatalf("output = %q, want %q", output, want)
	}
}
