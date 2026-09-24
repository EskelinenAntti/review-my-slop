package ui

import (
	"github.com/eskelinenantti/review-my-slop/internal/comments"
	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

type SaveCommentFunc func(comments.Comment, patch.Patch) (comments.Comment, error)

// mode is test-only scaffolding for the unexported model field.
type mode = uint8

type InitialLayout struct {
	SideBySide     bool
	SaveSideBySide func(bool) error
	Size           Size
}

func New(p patch.Patch, items []comments.Comment, save SaveCommentFunc, layout InitialLayout) Model {
	size := layout.Size
	if size.Width <= 0 || size.Height <= 0 {
		size = DefaultSize
	}
	m := Model{
		review:     reviewState{patch: p},
		comments:   commentState{items: items, editIndex: -1},
		width:      size.Width,
		height:     size.Height,
		save:       save,
		saveLayout: layout.SaveSideBySide,
		dark:       true,
	}
	review := &m.review
	review.sideBySide = layout.SideBySide
	review.view = newDiffView(p, m.dark, m.sideBySideActive())
	review.viewport = review.view.Resize(Viewport{}, m.width, m.screenBodyHeight())
	review.cursor, _ = review.view.First()
	return m
}

func (m *Model) setSideBySide(enabled bool) {
	review := &m.review
	wasActive := m.sideBySideActive()
	review.sideBySide = enabled
	if wasActive != m.sideBySideActive() {
		m.rebuildView(review.patch)
	}
}
