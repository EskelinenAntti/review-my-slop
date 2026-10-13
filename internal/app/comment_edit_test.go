package app

import (
	"errors"
	"fmt"
	"slices"
	"testing"

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
	testutil.RequireEqual(t, len(saved), 1,
		fmt.Sprintf("saved comments = %d", len(saved)),
	)

	anchor := saved[0].Anchor
	testutil.RequireEqual(t, anchor.FilePath, "main.go",
		fmt.Sprintf("anchor = %#v", anchor),
	)
	testutil.RequireEqual(t, anchor.OldStart, 2,
		fmt.Sprintf("anchor = %#v", anchor),
	)
	testutil.RequireEqual(t, anchor.NewStart, 2,
		fmt.Sprintf("anchor = %#v", anchor),
	)
	testutil.RequireEqual(t,
		slices.Equal(anchor.QuotedLines, []string{"-old()", "+new()"}), true,
		fmt.Sprintf("anchor = %#v", anchor),
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
	testutil.RequireEqual(t, m.edit.body, "",
		fmt.Sprintf("body=%q err=%v", m.edit.body, m.err),
	)
	testutil.RequireEqual(t, m.err != nil, true,
		fmt.Sprintf("body=%q err=%v", m.edit.body, m.err),
	)
	testutil.RequireEqual(t, m.err.Error(), "storage unavailable",
		fmt.Sprintf("body=%q err=%v", m.edit.body, m.err),
	)
}

func TestCommentRequiresEditor(t *testing.T) {
	t.Setenv("EDITOR", "")
	m := updateModel(t, testModel(coveragePatch(), nil, nil), textKey("c"))
	testutil.RequireEqual(t, m.err != nil, true,
		fmt.Sprintf("error = %v", m.err),
	)
	testutil.RequireEqual(t, m.err.Error(), "$EDITOR is not set",
		fmt.Sprintf("error = %v", m.err),
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
	testutil.RequireEqual(t, !called, true,
		fmt.Sprintf("called=%v comments=%d", called, len(m.comments.items)),
	)
	testutil.RequireEqual(t, len(m.comments.items), 0,
		fmt.Sprintf("called=%v comments=%d", called, len(m.comments.items)),
	)
}

func TestSelectionCannotCrossHunk(t *testing.T) {
	m := testModel(coveragePatch(), nil, nil)
	m = updateModel(t, m, textKey("v"))
	for range 10 {
		m = updateModel(t, m, textKey("j"))
	}
	_, line, _ := m.diffView.Current()
	testutil.RequireEqual(t, line.Text != "more()", true,
		"diffSelection crossed hunk",
	)
}
