package app

import (
	"errors"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/eskelinenantti/review-my-slop/internal/comments"
	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

func TestVisualSelectionCreatesMappedAnchorAndSubmits(t *testing.T) {
	t.Setenv(editorVariable, fixtureTrue)
	var saved []comments.Comment
	m := testModel(
		coveragePatch(),
		nil,
		func(
			stored comments.Comment,
			_ patch.Patch,
		) (
			comments.Comment,
			error,
		) {
			saved = append(saved, stored)
			stored.ID = "new"
			return stored, nil
		},
	)
	m = updateModel(t, m, textKey(keyJ))
	m = updateModel(t, m, textKey(selectionKey))
	m = updateModel(t, m, textKey(keyJ))
	m = updateModel(t, m, textKey(commentKey))
	_ = updateModel(t, m, commentEditorFinishedMsg{body: "fix both lines"})
	requireEqual(t, len(saved), 1, "saved comments = %d", len(saved))
	anchor := saved[0].Anchor
	requireEqual(
		t,
		anchor.FilePath == fixtureMainGo && anchor.OldStart == 2 &&
			anchor.NewStart == 2 &&
			slices.Equal(anchor.QuotedLines, []string{"-old()", "+new()"}),
		true,
		"anchor = %#v",
		anchor,
	)
}

func TestCommentSaveFailureClearsPendingEdit(t *testing.T) {
	t.Setenv(editorVariable, fixtureTrue)
	m := testModel(
		coveragePatch(),
		nil,
		func(comments.Comment, patch.Patch) (
			comments.Comment,
			error,
		) {
			return comments.Comment{}, errors.New("storage unavailable")
		},
	)
	m = updateModel(t, m, textKey(commentKey))
	m = updateModel(t, m, commentEditorFinishedMsg{body: "keep this"})
	requireEqual(
		t,
		m.edit.body == "" && m.err != nil &&
			m.err.Error() == "storage unavailable",
		true,
		"body=%q err=%v",
		m.edit.body,
		m.err,
	)
}

func TestCommentRequiresEditor(t *testing.T) {
	t.Setenv(editorVariable, "")
	m := updateModel(
		t,
		testModel(coveragePatch(), nil, nil),
		textKey(commentKey),
	)
	requireEqual(
		t,
		m.err != nil && m.err.Error() == "$EDITOR is not set", true,
		"error = %v",
		m.err,
	)
}

func TestEmptyNewCommentIsDiscarded(t *testing.T) {
	t.Setenv(editorVariable, fixtureTrue)
	called := false
	m := testModel(
		coveragePatch(),
		nil,
		func(
			stored comments.Comment,
			_ patch.Patch,
		) (
			comments.Comment,
			error,
		) {
			called = true
			return stored, nil
		},
	)
	m = updateModel(t, m, textKey(commentKey))
	m = updateModel(t, m, commentEditorFinishedMsg{body: " \n"})
	requireEqual(
		t,
		!called && len(m.comments.items) == 0, true,
		"called=%v comments=%d",
		called,
		len(m.comments.items),
	)
}

func TestOpenCurrentLineUsesEditorWithWorkingTreeLocation(t *testing.T) {
	t.Setenv(editorVariable, "printf")
	m := testModel(coveragePatch(), nil, nil)
	m.currentPatch.Root = "/tmp/repo with spaces"
	focusLine(t, &m, fixtureNew)
	cmd, err := m.openCurrentLine()
	requireEqual(
		t,
		err == nil && cmd != nil, true,
		"command=%v err=%v",
		cmd,
		err,
	)

	path, line, err := m.sourceLocation()
	requireEqual(
		t,
		err == nil && path == "/tmp/repo with spaces/main.go" && line == 2,
		true,
		"location=%q:%d err=%v",
		path,
		line,
		err,
	)
}

func TestOpenCurrentLineRequiresEditor(t *testing.T) {
	t.Setenv(editorVariable, "")
	m := updateModel(t, testModel(coveragePatch(), nil, nil), textKey("e"))
	requireEqual(
		t,
		m.err != nil && m.err.Error() == "$EDITOR is not set", true,
		"error = %v",
		m.err,
	)
}

func TestCommentsCanBeViewedEditedAndDeleted(t *testing.T) {
	t.Setenv(editorVariable, fixtureTrue)
	items := []comments.Comment{
		{ID: "one", Body: "old body"},
		{ID: "two", Body: "second"},
	}
	var persisted, deleted comments.Comment
	m := testModel(
		coveragePatch(),
		items,
		func(
			stored comments.Comment,
			_ patch.Patch,
		) (
			comments.Comment,
			error,
		) {
			persisted = stored
			return stored, nil
		},
	)
	m.setDelete(
		func(stored comments.Comment, _ patch.Patch) error {
			deleted = stored
			return nil
		},
	)
	m = updateModel(t, m, textKey(commentsKey))
	requireEqual(
		t,
		m.mode == modeComments && strings.Contains(m.render(), "old body"),
		true,
		valueFormat,
		"comments did not open",
	)
	m = updateModel(t, m, specialKey(tea.KeyEnter))
	m = updateModel(t, m, commentEditorFinishedMsg{body: "edited body"})
	requireEqual(
		t,
		persisted.ID == "one" && persisted.Body == "edited body", true,
		"persisted = %#v",
		persisted,
	)
	m = updateModel(t, m, textKey("D"))
	requireEqual(
		t,
		deleted.ID == "one" && len(m.comments.items) == 1, true,
		"deleted=%#v comments=%#v",
		deleted,
		m.comments.items,
	)
	m = updateModel(t, m, textKey("q"))
	requireEqual(
		t,
		m.mode == modeBrowse && !m.quitting, true,
		"mode=%v quitting=%v",
		m.mode,
		m.quitting,
	)
}

func TestOpeningCommentsReloadsPendingComments(t *testing.T) {
	m := testModel(
		coveragePatch(),
		[]comments.Comment{{ID: "read", Body: "already read"}},
		nil,
	)
	m.setLoadComments(func() ([]comments.Comment, error) { return nil, nil })

	next, cmd := m.Update(textKey(commentsKey))
	m = requireType[model](t, next)
	requireEqual(
		t,
		cmd != nil,
		true,
		valueFormat,
		"opening comments did not request a refresh",
	)
	m = updateModel(t, m, cmd())
	requireEqual(
		t,
		m.mode == modeComments && len(m.comments.items) == 0, true,
		"mode=%v comments=%#v",
		m.mode,
		m.comments.items,
	)
}

func TestCommentReloadFailurePreservesCurrentComments(t *testing.T) {
	m := testModel(
		coveragePatch(),
		[]comments.Comment{{ID: "keep", Body: "keep"}},
		nil,
	)
	m.setLoadComments(
		func() ([]comments.Comment, error) {
			return nil, errors.New("storage unavailable")
		},
	)

	next, cmd := m.Update(textKey(commentsKey))
	m = requireType[model](t, next)
	m = updateModel(t, m, cmd())
	requireEqual(
		t,
		len(m.comments.items) == 1 && m.err != nil &&
			m.err.Error() == "refresh comments: storage unavailable",
		true,
		"comments=%#v error=%v",
		m.comments.items,
		m.err,
	)
}

func TestEmptyEditedCommentIsDeleted(t *testing.T) {
	t.Setenv(editorVariable, fixtureTrue)
	m := testModel(
		coveragePatch(),
		[]comments.Comment{{ID: "one", Body: "old"}},
		nil,
	)
	deleted := false
	m.setDelete(
		func(comments.Comment, patch.Patch) error {
			deleted = true
			return nil
		},
	)
	m = updateModel(t, m, textKey(commentsKey))
	m = updateModel(t, m, specialKey(tea.KeyEnter))
	m = updateModel(t, m, commentEditorFinishedMsg{body: newline})
	requireEqual(
		t,
		deleted && len(m.comments.items) == 0, true,
		"deleted=%v comments=%d",
		deleted,
		len(m.comments.items),
	)
}

func TestCommentDeleteFailureKeepsCommentAndShowsError(t *testing.T) {
	m := testModel(
		coveragePatch(),
		[]comments.Comment{{ID: "one", Body: "keep"}},
		nil,
	)
	m.setDelete(
		func(comments.Comment, patch.Patch) error {
			return errors.New("delete failed")
		},
	)
	m = updateModel(t, m, textKey(commentsKey))
	m = updateModel(t, m, textKey("D"))
	requireEqual(
		t,
		len(m.comments.items) == 1 &&
			strings.Contains(ansi.Strip(m.render()), "delete failed"),
		true,
		"comments=%#v render=%q",
		m.comments.items,
		m.render(),
	)
}
