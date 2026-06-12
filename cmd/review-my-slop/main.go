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
	saveComment := func(stored review.StoredComment) (review.StoredComment, error) {
		if stored.BatchID != "" {
			return stored, store.UpdateComment(loaded.Repository, stored)
		}
		batch := review.Batch{
			ID:              fmt.Sprintf("%d", time.Now().UnixNano()),
			Repository:      loaded.Repository,
			DiffFingerprint: loaded.Fingerprint,
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
	program := tea.NewProgram(tui.New(loaded, comments, saveComment))
	_, err = program.Run()
	return err
}
