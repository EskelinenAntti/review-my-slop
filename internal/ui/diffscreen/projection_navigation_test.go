package diffscreen

import (
	"fmt"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestSplitViewWithOnlyDeletionsStartsInLeftPane(t *testing.T) {
	p := patch.Patch{
		Files: []patch.File{
			{
				DisplayPath: "deleted.go",
				Hunks: []patch.Hunk{
					{
						Header: "@@",
						Lines: []patch.Line{
							{Kind: patch.Deletion, Text: "gone", OldNumber: 1},
						},
					},
				},
			},
		},
	}
	v := newSplitPatchView(p, true)
	cursor, ok := v.firstCursor()
	testutil.RequireEqual(t, ok, true,
		fmt.Sprintf("first cursor = %#v, %v", cursor, ok),
	)
	testutil.RequireEqual(t, cursor.pane, left,
		fmt.Sprintf("first cursor = %#v, %v", cursor, ok),
	)

	line, _ := v.lineAtCursor(cursor)
	testutil.RequireEqual(t, line.Text, "gone",
		fmt.Sprintf("first line = %q", line.Text),
	)
}

func TestUnifiedNavigationSearchAndFileJumps(t *testing.T) {
	v := newUnifiedPatchView(twoFileReviewPatch(), true)
	firstCursor, ok := v.firstCursor()
	testutil.RequireEqual(t, ok, true, "First returned no cursor")

	line, _ := v.lineAtCursor(firstCursor)
	testutil.RequireEqual(t, line.Text, "before",
		fmt.Sprintf("first line = %q", line.Text),
	)

	next, ok := v.moveCursor(firstCursor, forward)
	testutil.RequireEqual(t, ok, true, "Move returned no cursor")

	line, _ = v.lineAtCursor(next)
	testutil.RequireEqual(t, line.Kind, patch.Deletion,
		fmt.Sprintf("next kind = %v", line.Kind),
	)

	match, ok := v.searchFromCursor("added", firstCursor, forward)
	testutil.RequireEqual(t, ok, true, "Search returned no cursor")

	line, _ = v.lineAtCursor(match)
	testutil.RequireEqual(t, line.Text, "added one",
		fmt.Sprintf("match = %q", line.Text),
	)

	jumped, ok := v.jumpToAdjacentFile(firstCursor, forward)
	testutil.RequireEqual(t, ok, true, "JumpFile returned no cursor")

	file, _ := v.fileAtCursor(jumped)
	testutil.RequireEqual(t, file.DisplayPath, "second.go",
		fmt.Sprintf("jumped file = %q", file.DisplayPath),
	)

	if _, ok := v.jumpToAdjacentFile(jumped, forward); ok {
		t.Fatal("JumpFile wrapped unexpectedly")
	}
}
