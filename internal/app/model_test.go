package app

import (
	"fmt"
	"path/filepath"
	"slices"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/comments"
	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func testModel(
	p patch.Patch,
	items []comments.Comment,
	save saveCommentFunc,
) model {
	return newModel(
		p,
		items,
		save,
		initialLayout{size: size{Width: 100, Height: 30}},
	)
}

func TestStoreBackedCommentLifecycle(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	store := comments.Store{Path: filepath.Join(t.TempDir(), "comments.db")}
	m, err := newWithStore(store, modelPatch(), nil, defaultSize)
	testutil.NoError(t, err)

	anchor := comments.Anchor{
		FilePath:    "main.go",
		NewStart:    2,
		QuotedLines: []string{"+new()"},
	}
	m.edit.body = "check this"
	m.edit.anchor = anchor
	m.finishCommentEdit()
	testutil.RequireEqual(t, m.err, nil, fmt.Sprintf("%s", m.err))

	loaded, err := m.load()
	testutil.RequireEqual(t, err, nil,
		fmt.Sprintf("loaded = %#v, error = %v", loaded, err),
	)
	testutil.RequireEqual(t, len(loaded), 1,
		fmt.Sprintf("loaded = %#v, error = %v", loaded, err),
	)
	testutil.RequireEqual(t, loaded[0].Repository, "/repo",
		fmt.Sprintf("loaded = %#v, error = %v", loaded, err),
	)
	testutil.RequireEqual(t, loaded[0].ID != "", true,
		fmt.Sprintf("loaded = %#v, error = %v", loaded, err),
	)
	testutil.RequireEqual(t,
		slices.Equal(loaded[0].Anchor.QuotedLines, anchor.QuotedLines), true,
		fmt.Sprintf("loaded = %#v, error = %v", loaded, err),
	)

	id := loaded[0].ID
	m.edit.index = 0
	m.edit.body = "edited"
	m.finishCommentEdit()
	testutil.RequireEqual(t, m.err, nil, fmt.Sprintf("%s", m.err))

	loaded, err = m.load()
	testutil.RequireEqual(t, err, nil,
		fmt.Sprintf("edited = %#v, error = %v", loaded, err),
	)
	testutil.RequireEqual(t, len(loaded), 1,
		fmt.Sprintf("edited = %#v, error = %v", loaded, err),
	)
	testutil.RequireEqual(t, loaded[0].ID, id,
		fmt.Sprintf("edited = %#v, error = %v", loaded, err),
	)
	testutil.RequireEqual(t, loaded[0].Body, "edited",
		fmt.Sprintf("edited = %#v, error = %v", loaded, err),
	)

	m.deleteComment(0)
	testutil.RequireEqual(t, m.err, nil, fmt.Sprintf("%s", m.err))

	loaded, err = m.load()
	testutil.RequireEqual(t, err, nil,
		fmt.Sprintf("remaining = %#v, error = %v", loaded, err),
	)
	testutil.RequireEqual(t, len(loaded), 0,
		fmt.Sprintf("remaining = %#v, error = %v", loaded, err),
	)
}

func TestCommentSaveUsesPatchAndPreservesAnchor(t *testing.T) {
	var savedPatch patch.Patch
	m := testModel(modelPatch(), nil, func(
		stored comments.Comment,
		p patch.Patch,
	) (comments.Comment, error) {
		savedPatch = p
		stored.ID = "1"
		return stored, nil
	})
	m.edit.body = "comment"
	m.edit.anchor = comments.Anchor{FilePath: "main.go"}
	m.finishCommentEdit()
	testutil.RequireEqual(t, savedPatch.Root, "/repo",
		fmt.Sprintf("saved patch/comments = %#v %#v", savedPatch,
			m.comments.items,
		),
	)
	testutil.RequireEqual(t, len(m.comments.items), 1,
		fmt.Sprintf("saved patch/comments = %#v %#v", savedPatch,
			m.comments.items,
		),
	)
	testutil.RequireEqual(t, m.comments.items[0].Anchor.FilePath, "main.go",
		fmt.Sprintf("saved patch/comments = %#v %#v", savedPatch,
			m.comments.items,
		),
	)
}

func modelPatch() patch.Patch {
	return patch.Patch{
		Root: "/repo",
		Files: []patch.File{
			{
				DisplayPath: "main.go",
				OldPath:     "main.go",
				NewPath:     "main.go",
				Hunks: []patch.Hunk{
					{
						Header: "@@ -1,2 +1,2 @@",
						Lines: []patch.Line{
							{
								Kind:      patch.Context,
								Text:      "keep()",
								OldNumber: 1,
								NewNumber: 1,
							},
							{Kind: patch.Deletion, Text: "old()", OldNumber: 2},
							{Kind: patch.Addition, Text: "new()", NewNumber: 2},
						},
					},
				},
			},
		},
	}
}
