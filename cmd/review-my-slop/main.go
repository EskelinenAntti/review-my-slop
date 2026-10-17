// Command review-my-slop provides an interactive diff review and feedback tool.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/eskelinenantti/review-my-slop/internal/app"
	"github.com/eskelinenantti/review-my-slop/internal/comments"
	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

const errorExitCode = 1

func main() {
	if err := run(context.Background(), os.Args[1:], os.Stdout); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "review-my-slop:", err)
		os.Exit(errorExitCode)
	}
}

func run(ctx context.Context, args []string, output io.Writer) error {
	command, err := parseCommand(args)
	if err != nil {
		return err
	}
	commentStore, err := comments.OpenDefault()
	if err != nil {
		return err
	}
	currentPatch, err := patch.Get(ctx, patch.Unstaged)
	if err != nil {
		return err
	}
	if command == "comments" {
		return commentStore.WritePending(output, currentPatch.Root)
	}
	return app.Run(ctx, currentPatch, commentStore, patch.Get)
}

func parseCommand(args []string) (string, error) {
	if len(args) == 0 {
		return "code", nil
	}
	if len(args) > 1 {
		return "", errors.New("usage: review-my-slop [code|comments]")
	}
	switch args[0] {
	case "code", "comments":
		return args[0], nil
	default:
		return "", fmt.Errorf(
			"unknown subcommand %q; usage: review-my-slop [code|comments]",
			args[0],
		)
	}
}
