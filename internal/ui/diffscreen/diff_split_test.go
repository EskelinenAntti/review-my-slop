package diffscreen

import (
	"fmt"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestSplitPairsUnequalChangeBlocksAndKeepsHunksSeparate(t *testing.T) {
	p := patch.Patch{
		Files: []patch.File{{DisplayPath: "file", Hunks: []patch.Hunk{
			{
				Header: "one",
				Lines: []patch.Line{
					{Kind: patch.Deletion, Text: "d1", OldNumber: 1},
					{Kind: patch.Deletion, Text: "d2", OldNumber: 2},
					{Kind: patch.Addition, Text: "a1", NewNumber: 1},
					{Kind: patch.Addition, Text: "a2", NewNumber: 2},
					{Kind: patch.Addition, Text: "a3", NewNumber: 3},
					{
						Kind:      patch.Context,
						Text:      "c",
						OldNumber: 3,
						NewNumber: 4,
					},
				},
			},
			{
				Header: "two",
				Lines: []patch.Line{
					{Kind: patch.Addition, Text: "separate", NewNumber: 5},
				},
			},
		}}},
	}
	v := newSideBySideView(p, true)
	var code []entry
	for _, current := range v.rows {
		if current.kind == lineRow {
			code = append(code, current)
		}
	}
	testutil.RequireEqual(t, len(code), 5,
		fmt.Sprintf("visual code entries = %d", len(code)),
	)
	testutil.RequireEqual(t, code[0].leftLine, 0,
		fmt.Sprintf("pairing = %#v", code[:3]),
	)
	testutil.RequireEqual(t, code[0].rightLine, 2,
		fmt.Sprintf("pairing = %#v", code[:3]),
	)
	testutil.RequireEqual(t, code[1].leftLine, 1,
		fmt.Sprintf("pairing = %#v", code[:3]),
	)
	testutil.RequireEqual(t, code[1].rightLine, 3,
		fmt.Sprintf("pairing = %#v", code[:3]),
	)
	testutil.RequireEqual(t, code[2].leftLine, -1,
		fmt.Sprintf("pairing = %#v", code[:3]),
	)
	testutil.RequireEqual(t, code[2].rightLine, 4,
		fmt.Sprintf("pairing = %#v", code[:3]),
	)
	testutil.RequireEqual(t, code[4].hunk != code[3].hunk, true,
		fmt.Sprintf("hunks were paired: %#v", code[4]),
	)
	testutil.RequireEqual(t, code[4].leftLine, -1,
		fmt.Sprintf("hunks were paired: %#v", code[4]),
	)
}

func TestSplitSelectionOnlyIncludesActivePane(t *testing.T) {
	v := newSideBySideView(testPatch(), true)
	first := mustFirst(t, v)
	removed, _ := v.search("removed one", first, forward)
	added, _ := v.search("added one", first, forward)
	left := beginSelection(removed)
	left, ok := v.extendSelection(left, removed)
	testutil.RequireEqual(t, ok, true,
		fmt.Sprintf("left lines = %#v", v.lines(left)),
	)
	testutil.RequireEqual(t, len(v.lines(left)), 1,
		fmt.Sprintf("left lines = %#v", v.lines(left)),
	)
	testutil.RequireEqual(t, v.lines(left)[0].Kind, patch.Deletion,
		fmt.Sprintf("left lines = %#v", v.lines(left)),
	)

	right := beginSelection(added)
	testutil.RequireEqual(t, len(v.lines(right)), 1,
		fmt.Sprintf("right lines = %#v", v.lines(right)),
	)
	testutil.RequireEqual(t, v.lines(right)[0].Kind, patch.Addition,
		fmt.Sprintf("right lines = %#v", v.lines(right)),
	)
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
	v := newSideBySideView(p, true)
	first := mustFirst(t, v)
	left, ok := v.switchPane(first, left)
	testutil.RequireEqual(t, ok, true, "did not find later left pane")

	line, _ := v.line(left)
	testutil.RequireEqual(t, line.Text, "both",
		fmt.Sprintf("later left line = %q", line.Text),
	)

	last, _ := v.last()
	right, ok := v.switchPane(last, right)
	testutil.RequireEqual(t, ok, true, "did not find earlier right pane")

	line, _ = v.line(right)
	testutil.RequireEqual(t, line.Text, "both",
		fmt.Sprintf("earlier right line = %q", line.Text),
	)
}

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
	v := newSideBySideView(p, true)
	cursor := mustFirst(t, v)
	if _, ok := v.switchPane(cursor, left); ok {
		t.Fatal("switched to empty pane")
	}
}
