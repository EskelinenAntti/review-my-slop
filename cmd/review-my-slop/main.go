package main

import (
	"context"
	"fmt"
	"os"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/anttieskelinen/review-my-slop/internal/gitdiff"
	"github.com/anttieskelinen/review-my-slop/internal/inbox"
	"github.com/anttieskelinen/review-my-slop/internal/review"
	"github.com/anttieskelinen/review-my-slop/internal/tui"
)

func main() {
	if err := run(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, "review-my-slop:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	current, err := os.Getwd()
	if err != nil {
		return err
	}
	loaded, err := (gitdiff.Loader{}).Load(ctx, current)
	if err != nil {
		return err
	}

	store, err := inbox.OpenDefault()
	if err != nil {
		return err
	}
	submit := func(comments []review.Comment) error {
		return store.Put(review.Batch{
			Repository:      loaded.Repository,
			DiffFingerprint: loaded.Fingerprint,
			CreatedAt:       time.Now().UTC(),
			Comments:        comments,
		})
	}
	program := tea.NewProgram(tui.New(loaded, submit))
	_, err = program.Run()
	return err
}
