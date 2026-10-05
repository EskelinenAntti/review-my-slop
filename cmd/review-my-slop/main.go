package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/eskelinenantti/review-my-slop/internal/app"
	"github.com/eskelinenantti/review-my-slop/internal/comments"
	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

func main() {
	if err := run(context.Background(), os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "review-my-slop:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, output io.Writer) error {
	if len(args) > 1 {
		return fmt.Errorf("usage: review-my-slop [code|comments]")
	}
	command := "code"
	if len(args) == 1 {
		command = args[0]
	}
	if command != "code" && command != "comments" {
		return fmt.Errorf("unknown subcommand %q; usage: review-my-slop [code|comments]", command)
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

func runComments(ctx context.Context, output io.Writer) error {
	return run(ctx, []string{"comments"}, output)
}
