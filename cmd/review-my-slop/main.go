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

func main() { os.Exit(exitCode(run(context.Background(), os.Args[1:], os.Stdout), os.Stderr)) }

func exitCode(err error, stderr io.Writer) int {
	if err != nil {
		fmt.Fprintln(stderr, "review-my-slop:", err)
		return 1
	}
	return 0
}

func run(ctx context.Context, args []string, output io.Writer) error {
	if len(args) == 0 {
		return runCode(ctx)
	}
	if len(args) > 1 {
		return fmt.Errorf("usage: review-my-slop [code|comments]")
	}
	return runSubcommand(ctx, args[0], output)
}

func runSubcommand(ctx context.Context, command string, output io.Writer) error {
	switch command {
	case "code":
		return runCode(ctx)
	case "comments":
		return runComments(ctx, output)
	default:
		return fmt.Errorf("unknown subcommand %q; usage: review-my-slop [code|comments]", command)
	}
}

func runCode(ctx context.Context) error {
	store, current, err := loadReview(ctx)
	if err != nil {
		return err
	}
	return app.Run(ctx, current, store, patch.Get)
}

func runComments(ctx context.Context, output io.Writer) error {
	store, current, err := loadReview(ctx)
	if err != nil {
		return err
	}
	return store.WritePending(output, current.Root)
}

func loadReview(ctx context.Context) (comments.Store, patch.Patch, error) {
	store, err := comments.OpenDefault()
	if err != nil {
		return comments.Store{}, patch.Patch{}, err
	}
	current, err := patch.Get(ctx, patch.Unstaged)
	return store, current, err
}
