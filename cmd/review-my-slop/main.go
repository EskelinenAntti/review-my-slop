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
	comments, err := store.ListComments(loaded.Repository)
	if err != nil {
		return err
	}
	loader := gitdiff.Loader{}
	saveComment := func(stored review.StoredComment, diff review.Diff) (review.StoredComment, error) {
		if stored.BatchID != "" {
			return stored, store.UpdateComment(diff.Repository, stored)
		}
		batch := review.Batch{
			ID:              fmt.Sprintf("%d", time.Now().UnixNano()),
			Repository:      diff.Repository,
			DiffFingerprint: diff.Fingerprint,
			CreatedAt:       time.Now().UTC(),
			Comments:        []review.Comment{stored.Comment},
		}
		if err := store.Put(batch); err != nil {
			return review.StoredComment{}, err
		}
		stored.BatchID = batch.ID
		stored.Index = 0
		return stored, nil
	}
	model := tui.New(loaded, comments, saveComment)
	model.SetRefresh(func() (review.Diff, error) {
		return loader.Load(ctx, current)
	})
	program := tea.NewProgram(model)
	_, err = program.Run()
	return err
}
