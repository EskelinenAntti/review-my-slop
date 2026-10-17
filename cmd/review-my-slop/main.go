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
	if len(args) > 1 {
		return errors.New("usage: review-my-slop [code|comments]")
	}
	command := "code"
	if len(args) == 1 {
		command = args[0]
	}
	switch command {
	case "code":
		return runCode(ctx)
	case "comments":
		return runComments(ctx, output)
	default:
		return fmt.Errorf(
			"unknown subcommand %q; usage: review-my-slop [code|comments]",
			command,
		)
	}
}

type currentReview struct {
	comments comments.Store
	patch    patch.Patch
}

func loadCurrentReview(ctx context.Context) (currentReview, error) {
	store, err := comments.OpenDefault()
	if err != nil {
		return currentReview{}, err
	}
	currentPatch, err := patch.Get(ctx, patch.Unstaged)
	if err != nil {
		return currentReview{}, err
	}
	return currentReview{comments: store, patch: currentPatch}, nil
}

func runCode(ctx context.Context) error {
	review, err := loadCurrentReview(ctx)
	if err != nil {
		return err
	}
	return app.Run(ctx, review.patch, review.comments, patch.Get)
}

func runComments(ctx context.Context, output io.Writer) error {
	review, err := loadCurrentReview(ctx)
	if err != nil {
		return err
	}
	return review.comments.WritePending(output, review.patch.Root)
}
