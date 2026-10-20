package app

import (
	"context"

	"github.com/eskelinenantti/review-my-slop/internal/comments"
	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/settings"
)

type saveCommentFunc func(comments.Comment) (comments.Comment, error)
type deleteCommentFunc func(comments.Comment) error
type loadCommentsFunc func() ([]comments.Comment, error)

// commentStore supplies the persistence operations used by the terminal client.
type commentStore interface {
	Add(comments.Comment) (comments.Comment, error)
	List(string) ([]comments.Comment, error)
	Update(comments.Comment) error
	Delete(string, string) error
}

func newWithStore(
	ctx context.Context,
	store commentStore,
	currentPatch patch.Patch,
	pendingComments []comments.Comment,
	dimensions terminalDimensions,
) (model, error) {
	preferences, err := settings.Load()
	if err != nil {
		return model{}, err
	}
	// Refreshes and patch-kind switches stay in the session's repository.
	repository := currentPatch.Root
	m := newModel(ctx, currentPatch, pendingComments, func(
		comment comments.Comment,
	) (comments.Comment, error) {
		comment.Repository = repository
		if comment.ID != "" {
			if err := store.Update(comment); err != nil {
				return comments.Comment{}, err
			}
			return comment, nil
		}
		return store.Add(comment)
	}, initialLayout{
		sideBySide: preferences.SideBySide,
		saveSideBySide: func(enabled bool) error {
			return settings.Save(settings.Preferences{SideBySide: enabled})
		},
		dimensions: dimensions,
	})
	m.delete = func(comment comments.Comment) error {
		return store.Delete(repository, comment.ID)
	}
	m.load = func() ([]comments.Comment, error) {
		return store.List(repository)
	}
	return m, nil
}
