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
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestVisualSelectionCreatesMappedAnchorAndSubmits(t *testing.T) {
	t.Setenv("EDITOR", "true")
	var saved []comments.Comment
	m := testModel(coveragePatch(), nil, func(
		stored comments.Comment,
		_ patch.Patch,
	) (comments.Comment, error) {
		saved = append(saved, stored)
		stored.ID = "new"
		return stored, nil
	})
	m = updateModel(t, m, textKey("j"))
	m = updateModel(t, m, textKey("v"))
	m = updateModel(t, m, textKey("j"))
	m = updateModel(t, m, textKey("c"))
	_ = updateModel(t, m, commentEditorFinishedMsg{body: "fix both lines"})
	testutil.RequireEqual(t, len(saved), 1, "saved comments = %d", len(saved))

	anchor := saved[0].Anchor
	testutil.RequireEqual(t, anchor.FilePath, "main.go", "anchor = %#v", anchor)
	testutil.RequireEqual(t, anchor.OldStart, 2, "anchor = %#v", anchor)
	testutil.RequireEqual(t, anchor.NewStart, 2, "anchor = %#v", anchor)
	testutil.RequireEqual(t,
		slices.Equal(anchor.QuotedLines, []string{"-old()", "+new()"}), true,
		"anchor = %#v", anchor,
	)
}

func TestCommentSaveFailureClearsPendingEdit(t *testing.T) {
	t.Setenv("EDITOR", "true")
	m := testModel(coveragePatch(), nil, func(
		comments.Comment,
		patch.Patch,
	) (comments.Comment, error) {
		return comments.Comment{}, errors.New("storage unavailable")
	})
	m = updateModel(t, m, textKey("c"))
	m = updateModel(t, m, commentEditorFinishedMsg{body: "keep this"})
	testutil.RequireEqual(t, m.edit.body, "", "body=%q err=%v", m.edit.body,
		m.err,
	)
	testutil.RequireEqual(t, m.err != nil, true, "body=%q err=%v", m.edit.body,
		m.err,
	)
	testutil.RequireEqual(t, m.err.Error(), "storage unavailable",
		"body=%q err=%v", m.edit.body, m.err,
	)
}

func TestCommentRequiresEditor(t *testing.T) {
	t.Setenv("EDITOR", "")
	m := updateModel(t, testModel(coveragePatch(), nil, nil), textKey("c"))
	testutil.RequireEqual(t, m.err != nil, true, "error = %v", m.err)
	testutil.RequireEqual(t, m.err.Error(), "$EDITOR is not set", "error = %v",
		m.err,
	)
}

func TestEmptyNewCommentIsDiscarded(t *testing.T) {
	t.Setenv("EDITOR", "true")
	called := false
	m := testModel(coveragePatch(), nil, func(
		stored comments.Comment,
		_ patch.Patch,
	) (comments.Comment, error) {
		called = true
		return stored, nil
	})
	m = updateModel(t, m, textKey("c"))
	m = updateModel(t, m, commentEditorFinishedMsg{body: " \n"})
	testutil.RequireEqual(t, !called, true, "called=%v comments=%d", called,
		len(m.comments.items),
	)
	testutil.RequireEqual(t, len(m.comments.items), 0, "called=%v comments=%d",
		called, len(m.comments.items),
	)
}

func TestCommentsCanBeViewedEditedAndDeleted(t *testing.T) {
	t.Setenv("EDITOR", "true")
	items := []comments.Comment{
		{ID: "one", Body: "old body"},
		{ID: "two", Body: "second"},
	}
	var persisted, deleted comments.Comment
	m := testModel(coveragePatch(), items, func(
		stored comments.Comment,
		_ patch.Patch,
	) (comments.Comment, error) {
		persisted = stored
		return stored, nil
	})
	m.delete = func(
		stored comments.Comment,
		_ patch.Patch,
	) error {
		deleted = stored
		return nil
	}
	m = updateModel(t, m, textKey("C"))
	testutil.RequireEqual(t, m.mode, modeComments, "%s",
		"comments did not open",
	)
	testutil.RequireEqual(t, strings.Contains(m.render(), "old body"), true,
		"%s", "comments did not open",
	)

	m = updateModel(t, m, specialKey(tea.KeyEnter))
	m = updateModel(t, m, commentEditorFinishedMsg{body: "edited body"})
	testutil.RequireEqual(t, persisted.ID, "one", "persisted = %#v", persisted)
	testutil.RequireEqual(t, persisted.Body, "edited body", "persisted = %#v",
		persisted,
	)

	m = updateModel(t, m, textKey("D"))
	testutil.RequireEqual(t, deleted.ID, "one", "deleted=%#v comments=%#v",
		deleted, m.comments.items,
	)
	testutil.RequireEqual(t, len(m.comments.items), 1,
		"deleted=%#v comments=%#v", deleted, m.comments.items,
	)

	m = updateModel(t, m, textKey("q"))
	testutil.RequireEqual(t, m.mode, modeBrowse, "mode=%v quitting=%v", m.mode,
		m.quitting,
	)
	testutil.RequireEqual(t, !m.quitting, true, "mode=%v quitting=%v", m.mode,
		m.quitting,
	)
}

func TestOpeningCommentsReloadsPendingComments(t *testing.T) {
	m := testModel(
		coveragePatch(),
		[]comments.Comment{{ID: "read", Body: "already read"}},
		nil,
	)
	m.load = func() ([]comments.Comment, error) { return nil, nil }

	next, cmd := m.Update(textKey("C"))
	m = requireModel(t, next)
	testutil.RequireEqual(t, cmd != nil, true, "%s",
		"opening comments did not request a refresh",
	)

	m = updateModel(t, m, cmd())
	testutil.RequireEqual(t, m.mode, modeComments, "mode=%v comments=%#v",
		m.mode, m.comments.items,
	)
	testutil.RequireEqual(t, len(m.comments.items), 0, "mode=%v comments=%#v",
		m.mode, m.comments.items,
	)
}

func TestCommentReloadFailurePreservesCurrentComments(t *testing.T) {
	m := testModel(
		coveragePatch(),
		[]comments.Comment{{ID: "keep", Body: "keep"}},
		nil,
	)
	m.load = func() ([]comments.Comment, error) {
		return nil, errors.New("storage unavailable")
	}

	next, cmd := m.Update(textKey("C"))
	m = requireModel(t, next)
	m = updateModel(t, m, cmd())
	testutil.RequireEqual(t, len(m.comments.items), 1, "comments=%#v error=%v",
		m.comments.items, m.err,
	)
	testutil.RequireEqual(t, m.err != nil, true, "comments=%#v error=%v",
		m.comments.items, m.err,
	)
	testutil.RequireEqual(t, m.err.Error(),
		"refresh comments: storage unavailable", "comments=%#v error=%v",
		m.comments.items, m.err,
	)
}

func TestEmptyEditedCommentIsDeleted(t *testing.T) {
	t.Setenv("EDITOR", "true")
	m := testModel(
		coveragePatch(),
		[]comments.Comment{{ID: "one", Body: "old"}},
		nil,
	)
	deleted := false
	m.delete = func(comments.Comment, patch.Patch) error {
		deleted = true
		return nil
	}
	m = updateModel(t, m, textKey("C"))
	m = updateModel(t, m, specialKey(tea.KeyEnter))
	m = updateModel(t, m, commentEditorFinishedMsg{body: "\n"})
	testutil.RequireEqual(t, deleted, true, "deleted=%v comments=%d", deleted,
		len(m.comments.items),
	)
	testutil.RequireEqual(t, len(m.comments.items), 0, "deleted=%v comments=%d",
		deleted, len(m.comments.items),
	)
}

func TestCommentDeleteFailureKeepsCommentAndShowsError(t *testing.T) {
	m := testModel(
		coveragePatch(),
		[]comments.Comment{{ID: "one", Body: "keep"}},
		nil,
	)
	m.delete = func(comments.Comment, patch.Patch) error {
		return errors.New("delete failed")
	}
	m = updateModel(t, m, textKey("C"))
	m = updateModel(t, m, textKey("D"))
	testutil.RequireEqual(t, len(m.comments.items), 1, "comments=%#v render=%q",
		m.comments.items, m.render(),
	)
	testutil.RequireEqual(t,
		strings.Contains(ansi.Strip(m.render()), "delete failed"), true,
		"comments=%#v render=%q", m.comments.items, m.render(),
	)
}

func TestCommentAfterRefreshUsesCurrentPatch(t *testing.T) {
	t.Setenv("EDITOR", "true")
	var saved patch.Patch
	m := testModel(coveragePatch(), nil, func(
		stored comments.Comment,
		p patch.Patch,
	) (comments.Comment, error) {
		saved = p
		return stored, nil
	})
	refreshed := coveragePatch()
	refreshed.Files[0].Metadata = []string{"refreshed"}
	m = updateModel(t, m, patchRefreshFinishedMsg{patch: refreshed})
	m = updateModel(t, m, textKey("c"))
	_ = updateModel(t, m, commentEditorFinishedMsg{body: "comment"})
	testutil.RequireEqual(t,
		slices.Equal(saved.Files[0].Metadata, []string{"refreshed"}), true,
		"metadata=%q", saved.Files[0].Metadata,
	)
}
