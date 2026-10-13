package app

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/eskelinenantti/review-my-slop/internal/comments"
	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestCommentsCanBeViewedEditedAndDeleted(t *testing.T) {
	t.Setenv("EDITOR", "true")
	items := []comments.Comment{
		{ID: "one", Body: "old body"},
		{ID: "two", Body: "second"},
	}
	var persisted, deleted comments.Comment
	m := newTestModel(multiHunkPatch(), items, func(
		stored comments.Comment,
		_ patch.Patch,
	) (comments.Comment, error) {
		persisted = stored
		return stored, nil
	})
	m.setCommentDelete(
		func(
			stored comments.Comment,
			_ patch.Patch,
		) error {
			deleted = stored
			return nil
		},
	)
	m = applyTestMessage(t, m, textKeyMessage("C"))
	testutil.RequireEqual(t, m.mode, modeComments, "comments did not open")
	testutil.RequireEqual(t, strings.Contains(m.render(), "old body"), true,
		"comments did not open",
	)

	m = applyTestMessage(t, m, specialKeyMessage(tea.KeyEnter))
	m = applyTestMessage(t, m, commentEditorFinishedMsg{body: "edited body"})
	testutil.RequireEqual(t, persisted.ID, "one",
		fmt.Sprintf("persisted = %#v", persisted),
	)
	testutil.RequireEqual(t, persisted.Body, "edited body",
		fmt.Sprintf("persisted = %#v", persisted),
	)

	m = applyTestMessage(t, m, textKeyMessage("D"))
	testutil.RequireEqual(t, deleted.ID, "one",
		fmt.Sprintf("deleted=%#v comments=%#v", deleted, m.comments.items),
	)
	testutil.RequireEqual(t, len(m.comments.items), 1,
		fmt.Sprintf("deleted=%#v comments=%#v", deleted, m.comments.items),
	)

	m = applyTestMessage(t, m, textKeyMessage("q"))
	testutil.RequireEqual(t, m.mode, modeBrowse,
		fmt.Sprintf("mode=%v quitting=%v", m.mode, m.quitting),
	)
	testutil.RequireEqual(t, !m.quitting, true,
		fmt.Sprintf("mode=%v quitting=%v", m.mode, m.quitting),
	)
}

func TestOpeningCommentsReloadsPendingComments(t *testing.T) {
	m := newTestModel(
		multiHunkPatch(),
		[]comments.Comment{{ID: "read", Body: "already read"}},
		nil,
	)
	m.setCommentReload(func() ([]comments.Comment, error) { return nil, nil })

	next, cmd := m.Update(textKeyMessage("C"))
	m = requireAppModel(t, next)
	testutil.RequireEqual(t, cmd != nil, true,
		"opening comments did not request a refresh",
	)

	m = applyTestMessage(t, m, cmd())
	testutil.RequireEqual(t, m.mode, modeComments,
		fmt.Sprintf("mode=%v comments=%#v", m.mode, m.comments.items),
	)
	testutil.RequireEqual(t, len(m.comments.items), 0,
		fmt.Sprintf("mode=%v comments=%#v", m.mode, m.comments.items),
	)
}

func TestCommentReloadFailurePreservesCurrentComments(t *testing.T) {
	m := newTestModel(
		multiHunkPatch(),
		[]comments.Comment{{ID: "keep", Body: "keep"}},
		nil,
	)
	m.setCommentReload(
		func() (
			[]comments.Comment,
			error,
		) {
			return nil, errors.New("storage unavailable")
		},
	)

	next, cmd := m.Update(textKeyMessage("C"))
	m = requireAppModel(t, next)
	m = applyTestMessage(t, m, cmd())
	testutil.RequireEqual(t, len(m.comments.items), 1,
		fmt.Sprintf("comments=%#v error=%v", m.comments.items, m.err),
	)
	testutil.RequireEqual(t, m.err != nil, true,
		fmt.Sprintf("comments=%#v error=%v", m.comments.items, m.err),
	)
	testutil.RequireEqual(t, m.err.Error(),
		"refresh comments: storage unavailable",
		fmt.Sprintf("comments=%#v error=%v", m.comments.items, m.err),
	)
}

func TestEmptyEditedCommentIsDeleted(t *testing.T) {
	t.Setenv("EDITOR", "true")
	m := newTestModel(
		multiHunkPatch(),
		[]comments.Comment{{ID: "one", Body: "old"}},
		nil,
	)
	deleted := false
	m.setCommentDelete(
		func(
			comments.Comment,
			patch.Patch,
		) error {
			deleted = true
			return nil
		},
	)
	m = applyTestMessage(t, m, textKeyMessage("C"))
	m = applyTestMessage(t, m, specialKeyMessage(tea.KeyEnter))
	m = applyTestMessage(t, m, commentEditorFinishedMsg{body: "\n"})
	testutil.RequireEqual(t, deleted, true,
		fmt.Sprintf("deleted=%v comments=%d", deleted, len(m.comments.items)),
	)
	testutil.RequireEqual(t, len(m.comments.items), 0,
		fmt.Sprintf("deleted=%v comments=%d", deleted, len(m.comments.items)),
	)
}

func TestCommentDeleteFailureKeepsCommentAndShowsError(t *testing.T) {
	m := newTestModel(
		multiHunkPatch(),
		[]comments.Comment{{ID: "one", Body: "keep"}},
		nil,
	)
	m.setCommentDelete(
		func(
			comments.Comment,
			patch.Patch,
		) error {
			return errors.New("delete failed")
		},
	)
	m = applyTestMessage(t, m, textKeyMessage("C"))
	m = applyTestMessage(t, m, textKeyMessage("D"))
	testutil.RequireEqual(t, len(m.comments.items), 1,
		fmt.Sprintf("comments=%#v render=%q", m.comments.items, m.render()),
	)
	testutil.RequireEqual(t,
		strings.Contains(ansi.Strip(m.render()), "delete failed"), true,
		fmt.Sprintf("comments=%#v render=%q", m.comments.items, m.render()),
	)
}
