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

func main() {
	if err := run(context.Background(), os.Args[one:], os.Stdout); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "review-my-slop:", err)
		os.Exit(one)
	}
}

func run(ctx context.Context, args []string, output io.Writer) error {
	if len(args) == zero {
		return runCode(ctx)
	}
	if len(args) > one {
		return errors.New("usage: review-my-slop [code|comments]")
	}
	switch args[zero] {
	case "code":
		return runCode(ctx)
	case "comments":
		return runComments(ctx, output)
	default:
		return fmt.Errorf(
			"unknown subcommand %q; usage: review-my-slop [code|comments]",
			args[zero],
		)
	}
}

func runCode(ctx context.Context) error {
	commentStore, err := comments.OpenDefault()
	if err != nil {
		return err
	}
	currentPatch, err := patch.Get(ctx, patch.Unstaged)
	if err != nil {
		return err
	}
	return app.Run(ctx, currentPatch, commentStore, patch.Get)
}

func runComments(ctx context.Context, output io.Writer) error {
	commentStore, err := comments.OpenDefault()
	if err != nil {
		return err
	}
	currentPatch, err := patch.Get(ctx, patch.Unstaged)
	if err != nil {
		return err
	}
	return commentStore.WritePending(output, currentPatch.Root)
}
