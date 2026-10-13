package diffscreen

import (
	"strings"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func mustFirstCursor(t *testing.T, v *patchProjection) patchCursor {
	t.Helper()
	cursor, ok := v.firstCursor()
	testutil.RequireEqual(t, ok, true, "no first cursor")

	return cursor
}

func twoFileReviewPatch() patch.Patch {
	return patch.Patch{Root: "/repo", Files: []patch.File{
		{
			DisplayPath: "first.go",
			OldPath:     "first.go",
			NewPath:     "first.go",
			Hunks: []patch.Hunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patch.Line{
				{
					Kind:      patch.Context,
					Text:      "before",
					OldNumber: 1,
					NewNumber: 1,
				},
				{Kind: patch.Deletion, Text: "removed one", OldNumber: 2},
				{Kind: patch.Deletion, Text: "removed two", OldNumber: 3},
				{Kind: patch.Addition, Text: "added one", NewNumber: 2},
				{
					Kind:      patch.Context,
					Text:      "after",
					OldNumber: 4,
					NewNumber: 3,
				},
			}}},
		},
		{
			DisplayPath: "second.go",
			OldPath:     "second.go",
			NewPath:     "second.go",
			Hunks: []patch.Hunk{
				{
					Header: "@@ -1 +1 @@",
					Lines: []patch.Line{
						{Kind: patch.Addition, Text: "other", NewNumber: 1},
					},
				},
			},
		},
	}}
}

func manyContextLinesPatch() patch.Patch {
	lines := make([]patch.Line, 20)
	for index := range lines {
		lines[index] = patch.Line{
			Kind:      patch.Context,
			Text:      strings.Repeat("long", 20),
			OldNumber: patch.LineNumber(index + 1),
			NewNumber: patch.LineNumber(index + 1),
		}
	}
	return patch.Patch{
		Files: []patch.File{
			{
				DisplayPath: "long.go",
				Hunks:       []patch.Hunk{{Header: "@@", Lines: lines}},
			},
		},
	}
}

func newUnifiedPatchView(p patch.Patch, dark bool) *patchProjection {
	return newPatchProjection(p, Options{Dark: dark})
}

func newSplitPatchView(p patch.Patch, dark bool) *patchProjection {
	return newPatchProjection(p, Options{SideBySide: true, Dark: dark})
}

func patchWithLines(lines []patch.Line) patch.Patch {
	return patchWithHunks([]patch.Hunk{{Header: "@@", Lines: lines}})
}

func patchWithHunks(hunks []patch.Hunk) patch.Patch {
	return patch.Patch{Files: []patch.File{
		{OldPath: "file.go", NewPath: "file.go", Hunks: hunks},
	}}
}

func pointerTo[T any](value T) *T { return &value }

func longCodeLinePatch() patch.Patch {
	return patch.Patch{
		Files: []patch.File{{
			DisplayPath: "long",
			Hunks: []patch.Hunk{{
				Header: "@@",
				Lines: []patch.Line{{
					Kind: patch.Context,
					Text: "abcdefghijklmnopqrstuvwxyz" +
						"0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ",
					OldNumber: 1,
					NewNumber: 1,
				}},
			}},
		}},
	}
}
