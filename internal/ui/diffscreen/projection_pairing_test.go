package diffscreen

import (
	"fmt"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestSplitPairsChangeBlocksAndSupportsEmptyPanes(t *testing.T) {
	v := newSplitPatchView(twoFileReviewPatch(), true)
	firstCursor, _ := v.firstCursor()
	removed, _ := v.moveCursor(firstCursor, forward)
	testutil.RequireEqual(t, removed.pane, right,
		fmt.Sprintf("initial pane = %v", removed.pane),
	)

	removed, ok := v.switchCursorPane(removed, left)
	testutil.RequireEqual(t, ok, true, "could not switch to deletion pane")

	line, _ := v.lineAtCursor(removed)
	testutil.RequireEqual(t, line.Text, "removed one",
		fmt.Sprintf("left line = %q", line.Text),
	)

	added, ok := v.switchCursorPane(removed, right)
	testutil.RequireEqual(t, ok, true, "paired addition missing")

	line, _ = v.lineAtCursor(added)
	testutil.RequireEqual(t, line.Text, "added one",
		fmt.Sprintf("right line = %q", line.Text),
	)

	secondRemoved, _ := v.moveCursor(removed, forward)
	if _, ok := v.switchCursorPane(secondRemoved, right); !ok {
		t.Fatal("pane switch should find a nearby right line")
	}

	viewport := v.newViewport(100, 20)
	rendered := v.renderProjection(viewport, added, nil)
	testutil.RequireEqual(t, strings.Contains(rendered, "removed one"), true,
		fmt.Sprintf("paired render missing lines: %q", rendered),
	)
	testutil.RequireEqual(t, strings.Contains(rendered, "added one"), true,
		fmt.Sprintf("paired render missing lines: %q", rendered),
	)

	for _, row := range strings.Split(rendered, "\n") {
		testutil.RequireEqual(t,
			!strings.Contains(row, " │ ") || lipgloss.Width(row) == 100, true,
			fmt.Sprintf("rendered width = %d", lipgloss.Width(row)),
		)
	}
}

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
	v := newSplitPatchView(p, true)
	var code []displayRow
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
