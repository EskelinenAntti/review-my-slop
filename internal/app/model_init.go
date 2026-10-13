package app

import (
	"context"

	"github.com/eskelinenantti/review-my-slop/internal/comments"
	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/settings"
	"github.com/eskelinenantti/review-my-slop/internal/ui/commentscreen"
	"github.com/eskelinenantti/review-my-slop/internal/ui/diffscreen"
	"github.com/eskelinenantti/review-my-slop/internal/ui/keymap"
)

func newModel(
	currentPatch patch.Patch,
	initialComments []comments.Comment,
	saveCommentToStore saveCommentToStoreFunc,
	layout initialLayout,
) model {
	size := layout.size
	if size.Width <= 0 || size.Height <= 0 {
		size = defaultSize
	}
	m := model{
		ctx:                context.Background(),
		currentPatch:       currentPatch,
		comments:           commentState{items: initialComments},
		commentView:        commentscreen.New(initialComments),
		edit:               commentEdit{index: -1},
		width:              size.Width,
		height:             size.Height,
		saveCommentToStore: saveCommentToStore,
		saveLayout:         layout.SaveSideBySide,
		patchViewOptions: diffscreen.Options{
			SideBySide: layout.SideBySide,
			Dark:       true,
		},
		kind: currentPatch.Kind,
		keys: keymap.New(
			keymap.Sequence{
				Prefix:         "g",
				Keys:           []string{"g"},
				RetryUnmatched: true,
			},
			keymap.Sequence{Prefix: "z", Keys: []string{"z", "t", "b"}},
			keymap.Sequence{Prefix: "[", Keys: []string{"f"}},
			keymap.Sequence{Prefix: "]", Keys: []string{"f"}},
			keymap.Sequence{
				Prefix: "ctrl+w",
				Keys:   []string{"h", "l", "ctrl+w"},
			},
		),
	}
	m.patchView = diffscreen.New(currentPatch, m.patchViewOptions)
	m.resizeScreens()
	return m
}

func newModelWithStore(
	store commentStore,
	currentPatch patch.Patch,
	initialComments []comments.Comment,
	dimensions size,
) (model, error) {
	preferences, err := settings.Load()
	if err != nil {
		return model{}, err
	}
	m := newModel(currentPatch, initialComments, func(
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
		SideBySide: preferences.SideBySide,
		SaveSideBySide: func(enabled bool) error {
			return settings.Save(settings.Preferences{SideBySide: enabled})
		},
		size: dimensions,
	})
	m.setCommentDelete(
		func(comment comments.Comment, current patch.Patch) error {
			return store.Delete(current.Root, comment.ID)
		},
	)
	m.setCommentReload(func() ([]comments.Comment, error) {
		return store.List(currentPatch.Root)
	})
	return m, nil
}

func (m *model) setPatchRefresh(refreshPatch refreshPatchFunc) {
	m.refreshPatch = refreshPatch
}

func (m *model) setCommentDelete(
	deleteCommentFromStore deleteCommentFromStoreFunc,
) {
	m.deleteCommentFromStore = deleteCommentFromStore
}

func (m *model) setCommentReload(reloadComments reloadCommentsFunc) {
	m.reloadComments = reloadComments
}
