package app

import (
	"context"

	"github.com/eskelinenantti/review-my-slop/internal/comments"
	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/settings"
)

type saveCommentFunc func(
	comments.Comment,
	patch.Patch,
) (comments.Comment, error)
type deleteCommentFunc func(comments.Comment, patch.Patch) error
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
	m := newModel(ctx, currentPatch, pendingComments, func(
		comment comments.Comment,
		current patch.Patch,
	) (comments.Comment, error) {
		comment.Repository = current.Root
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
	m.delete = func(comment comments.Comment, current patch.Patch) error {
		return store.Delete(current.Root, comment.ID)
	}
	m.load = func() ([]comments.Comment, error) {
		return store.List(currentPatch.Root)
	}
	return m, nil
}
