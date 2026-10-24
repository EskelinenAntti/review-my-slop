package app

import (
	"errors"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/comments"
	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestEditSelectedCommentStartup(t *testing.T) {
	items := []comments.Comment{
		{ID: "first", Body: "first comment"},
		{
			ID:     "second",
			Body:   "selected comment",
			Anchor: comments.Anchor{FilePath: "selected.go", NewStart: 7},
		},
	}
	for _, tc := range []struct {
		name      string
		items     []comments.Comment
		editor    string
		wantErr   string
		wantCmd   bool
		wantIndex int
	}{
		{name: "empty list", wantIndex: -1},
		{
			name:      "missing editor clears edit",
			items:     items,
			wantErr:   "$EDITOR is not set",
			wantIndex: -1,
		},
		{
			name:      "selected comment starts editor",
			items:     items,
			editor:    "true",
			wantCmd:   true,
			wantIndex: 1,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("EDITOR", tc.editor)
			m := testModel(modelPatch(), tc.items, nil)
			m.mode = modeComments
			m.commentView.Move(1)

			next, cmd := m.editSelectedComment()
			m = requireModel(t, next)
			testutil.RequireEqual(t, cmd != nil, tc.wantCmd, "command = %v",
				cmd != nil,
			)
			testutil.RequireEqual(t, m.mode, modeComments, "mode = %v", m.mode)
			testutil.RequireEqual(t, m.edit.index, tc.wantIndex, "edit = %#v",
				m.edit,
			)
			if tc.wantErr != "" {
				if m.err == nil {
					t.Fatal("expected editor startup error")
				}
				testutil.RequireEqual(t, m.err.Error(), tc.wantErr,
					"error = %v", m.err,
				)
			} else {
				testutil.NoError(t, m.err)
			}
			if tc.wantCmd {
				testutil.RequireEqual(t, m.edit.body, items[1].Body,
					"edit = %#v", m.edit,
				)
				testutil.RequireEqual(t, m.edit.anchor.FilePath, "selected.go",
					"edit = %#v", m.edit,
				)
				testutil.RequireEqual(t, m.edit.anchor.NewStart, 7,
					"edit = %#v", m.edit,
				)
			} else {
				testutil.RequireEqual(t, m.edit.body, "", "edit = %#v", m.edit)
				testutil.RequireEqual(t, m.edit.anchor.FilePath, "",
					"edit = %#v", m.edit,
				)
			}
		})
	}
}

func TestCommentEditorFailureClearsEditWithoutSaving(t *testing.T) {
	item := comments.Comment{ID: "existing", Body: "original body"}
	m := testModel(modelPatch(), []comments.Comment{item}, func(
		comment comments.Comment,
	) (comments.Comment, error) {
		t.Fatal("failed editor must not save a comment")
		return comment, nil
	})
	m.edit = commentEdit{index: 0, body: item.Body, anchor: item.Anchor}
	editorErr := errors.New("editor failed")

	next, cmd := m.Update(commentEditorFinishedMsg{
		body: "partial edit",
		err:  editorErr,
	})
	m = requireModel(t, next)
	if cmd != nil {
		t.Fatal("failed editor returned a command")
	}
	if !errors.Is(m.err, editorErr) {
		t.Fatalf("error = %v, want %v", m.err, editorErr)
	}
	testutil.RequireEqual(t, m.edit.index, -1, "edit = %#v", m.edit)
	testutil.RequireEqual(t, m.edit.body, "", "edit = %#v", m.edit)
	testutil.RequireEqual(t, m.comments.items[0].Body, item.Body,
		"stored comment = %#v", m.comments.items[0],
	)
}

func TestSourceEditorFailureDoesNotRefresh(t *testing.T) {
	m := testModel(modelPatch(), nil, nil)
	m.refresh = func(patch.Kind) (patch.Patch, error) {
		t.Fatal("failed editor must not refresh the diff")
		return patch.Patch{}, nil
	}
	editorErr := errors.New("editor failed")

	next, cmd := m.Update(sourceEditorFinishedMsg{err: editorErr})
	m = requireModel(t, next)
	if cmd != nil {
		t.Fatal("failed editor returned a refresh command")
	}
	if !errors.Is(m.err, editorErr) {
		t.Fatalf("error = %v, want wrapped %v", m.err, editorErr)
	}
	testutil.RequireEqual(t, m.err.Error(), "editor: editor failed",
		"error = %v", m.err,
	)
}
