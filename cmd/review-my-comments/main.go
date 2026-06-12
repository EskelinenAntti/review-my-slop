package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/anttieskelinen/review-my-slop/internal/gitdiff"
	"github.com/anttieskelinen/review-my-slop/internal/inbox"
)

func main() {
	if err := run(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, "review-my-comments:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	current, err := os.Getwd()
	if err != nil {
		return err
	}
	return runAt(ctx, current, os.Stdout)
}

func runAt(ctx context.Context, current string, output io.Writer) error {
	root, err := (gitdiff.Loader{}).Root(ctx, current)
	if err != nil {
		return err
	}
	store, err := inbox.OpenDefault()
	if err != nil {
		return err
	}
	taken, err := store.Peek(root)
	if err != nil {
		return err
	}
	if err := inbox.WritePrompt(output, taken.Batches); err != nil {
		return err
	}
	return store.Delete(taken)
}
