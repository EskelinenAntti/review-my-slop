package diffscreen

import (
	"fmt"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestSplitPaneSwitchDoesNothingWhenTargetPaneIsEmpty(t *testing.T) {
	p := patch.Patch{
		Files: []patch.File{
			{
				DisplayPath: "file",
				Hunks: []patch.Hunk{
					{
						Header: "@@",
						Lines: []patch.Line{
							{Kind: patch.Addition, Text: "one", NewNumber: 1},
							{Kind: patch.Addition, Text: "two", NewNumber: 2},
						},
					},
				},
			},
		},
	}
	v := newSplitPatchView(p, true)
	cursor := mustFirstCursor(t, v)
	if _, ok := v.switchCursorPane(cursor, left); ok {
		t.Fatal("switched to empty pane")
	}
}

func TestSplitPaneSwitchingFindsRowsAboveAndBelowEmptyTargets(t *testing.T) {
	p := patch.Patch{
		Files: []patch.File{
			{
				DisplayPath: "file",
				Hunks: []patch.Hunk{{Header: "@@", Lines: []patch.Line{
					{Kind: patch.Addition, Text: "right", NewNumber: 1},
					{
						Kind:      patch.Context,
						Text:      "both",
						OldNumber: 1,
						NewNumber: 2,
					},
					{Kind: patch.Deletion, Text: "left", OldNumber: 2},
				}}},
			},
		},
	}
	v := newSplitPatchView(p, true)
	firstCursor := mustFirstCursor(t, v)
	left, ok := v.switchCursorPane(firstCursor, left)
	testutil.RequireEqual(t, ok, true, "did not find later left pane")

	line, _ := v.lineAtCursor(left)
	testutil.RequireEqual(t, line.Text, "both",
		fmt.Sprintf("later left line = %q", line.Text),
	)

	lastCursor, _ := v.lastCursor()
	right, ok := v.switchCursorPane(lastCursor, right)
	testutil.RequireEqual(t, ok, true, "did not find earlier right pane")

	line, _ = v.lineAtCursor(right)
	testutil.RequireEqual(t, line.Text, "both",
		fmt.Sprintf("earlier right line = %q", line.Text),
	)
}
