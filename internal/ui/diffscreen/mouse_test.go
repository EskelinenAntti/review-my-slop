package diffscreen

import (
	"strings"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestClickAndDragSelectCode(t *testing.T) {
	v := New(patch.Patch{Root: "/repo", Files: []patch.File{{DisplayPath: "first.go", OldPath: "first.go", NewPath: "first.go", Hunks: []patch.Hunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "before", OldNumber: 1, NewNumber: 1}, {Kind: patch.Deletion, Text: "removed one", OldNumber: 2}, {Kind: patch.Deletion, Text: "removed two", OldNumber: 3}, {Kind: patch.Addition, Text: "added one", NewNumber: 2}, {Kind: patch.Context, Text: "after", OldNumber: 4, NewNumber: 3}}}}}, {DisplayPath: "second.go", OldPath: "second.go", NewPath: "second.go", Hunks: []patch.Hunk{{Header: "@@ -1 +1 @@", Lines: []patch.Line{{Kind: patch.Addition, Text: "other", NewNumber: 1}}}}}}}, Options{})
	v.Resize(80, 20)
	testutil.RequireEqual(t, v.BeginDrag(20, 4), true, "%s",
		"code click was ignored",
	)

	_, line, _ := v.Current()
	testutil.RequireEqual(t, line.Text, "removed one", "%s",
		"click did not focus code without selecting a range",
	)
	testutil.RequireEqual(t, v.selection, nil, "%s",
		"click did not focus code without selecting a range",
	)

	v.DragTo(30, 6)
	v.EndDrag()
	_, lines, ok := v.Selected()
	testutil.RequireEqual(t, ok, true, "drag selection = %#v", lines)
	testutil.RequireEqual(t, len(lines), 3, "drag selection = %#v", lines)
	testutil.RequireEqual(t, lines[0].Text, "removed one",
		"drag selection = %#v", lines,
	)
	testutil.RequireEqual(t, lines[2].Text, "added one", "drag selection = %#v",
		lines,
	)

	end := v.cursor
	v.DragTo(20, 3)
	testutil.RequireEqual(t, v.cursor, end, "%s",
		"motion after release changed selection",
	)
	testutil.RequireEqual(t, v.BeginDrag(20, 6), true, "%s",
		"new click did not clear old selection",
	)
	testutil.RequireEqual(t, v.selection, nil, "%s",
		"new click did not clear old selection",
	)

	v.DragTo(20, 3)
	_, lines, ok = v.Selected()
	testutil.RequireEqual(t, ok, true, "backward drag = %#v", lines)
	testutil.RequireEqual(t, len(lines), 4, "backward drag = %#v", lines)

	v.DragTo(20, 6)
	_, lines, _ = v.Selected()
	testutil.RequireEqual(t, len(lines), 1, "%s",
		"drag back to origin did not shrink selection",
	)
}

func TestMouseHitTestingMatchesScrolledProjection(t *testing.T) {
	for _, split := range []bool{false, true} {
		v := New((func() patch.Patch {
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
		})(), Options{SideBySide: split})
		v.Resize(121, 10)
		v.ScrollVertical(5)
		v.ScrollHorizontal(20)
		for _, point := range [][2]int{
			{0, 0},
			{0, 1},
			{0, 8},
			{0, 9},
			{-1, 3},
			{121, 3},
		} {
			testutil.RequireEqual(t, !v.BeginDrag(point[0], point[1]), true,
				"non-code point %v accepted", point,
			)
		}
		testutil.RequireEqual(t, v.BeginDrag(100, 2), true, "%s",
			"click did not account for viewport and sticky header",
		)
		testutil.RequireEqual(t, v.cursor.row, 5, "%s",
			"click did not account for viewport and sticky header",
		)
		testutil.RequireEqual(t, v.cursor.pane, right, "%s",
			"click did not account for viewport and sticky header",
		)

		v.DragTo(100, 4)
		_, lines, ok := v.Selected()
		testutil.RequireEqual(t, ok, true, "scrolled drag selection = %#v",
			lines,
		)
		testutil.RequireEqual(t, len(lines), 3, "scrolled drag selection = %#v",
			lines,
		)
	}
}

func TestSplitMouseRejectsDividerEmptyPaneAndCrossPaneDrag(t *testing.T) {
	v := New(patch.Patch{Root: "/repo", Files: []patch.File{{DisplayPath: "first.go", OldPath: "first.go", NewPath: "first.go", Hunks: []patch.Hunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "before", OldNumber: 1, NewNumber: 1}, {Kind: patch.Deletion, Text: "removed one", OldNumber: 2}, {Kind: patch.Deletion, Text: "removed two", OldNumber: 3}, {Kind: patch.Addition, Text: "added one", NewNumber: 2}, {Kind: patch.Context, Text: "after", OldNumber: 4, NewNumber: 3}}}}}, {DisplayPath: "second.go", OldPath: "second.go", NewPath: "second.go", Hunks: []patch.Hunk{{Header: "@@ -1 +1 @@", Lines: []patch.Line{{Kind: patch.Addition, Text: "other", NewNumber: 1}}}}}}}, Options{SideBySide: true})
	v.Resize(120, 20)
	for _, x := range []int{58, 59, 60} {
		testutil.RequireEqual(t, !v.BeginDrag(x, 3), true, "%s",
			"divider accepted",
		)
	}
	testutil.RequireEqual(t, !v.BeginDrag(80, 5), true, "%s",
		"empty right pane accepted",
	)
	testutil.RequireEqual(t, v.BeginDrag(10, 4), true, "%s",
		"deletion click did not focus old pane",
	)
	testutil.RequireEqual(t, v.cursor.pane, left, "%s",
		"deletion click did not focus old pane",
	)

	origin := v.cursor
	v.DragTo(80, 4)
	testutil.RequireEqual(t, v.cursor, origin, "%s", "drag crossed panes")
	testutil.RequireEqual(t, v.selection, nil, "%s", "drag crossed panes")

	v.DragTo(10, 5)
	_, lines, ok := v.Selected()
	testutil.RequireEqual(t, ok, true, "old-pane drag = %#v", lines)
	testutil.RequireEqual(t, len(lines), 2, "old-pane drag = %#v", lines)
	testutil.RequireEqual(t, lines[1].Text, "removed two",
		"old-pane drag = %#v", lines,
	)
}

func TestDragRejectsOtherHunksAndCancelsOnStateChanges(t *testing.T) {
	v := New(patch.Patch{Root: "/repo", Files: []patch.File{{DisplayPath: "first.go", OldPath: "first.go", NewPath: "first.go", Hunks: []patch.Hunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "before", OldNumber: 1, NewNumber: 1}, {Kind: patch.Deletion, Text: "removed one", OldNumber: 2}, {Kind: patch.Deletion, Text: "removed two", OldNumber: 3}, {Kind: patch.Addition, Text: "added one", NewNumber: 2}, {Kind: patch.Context, Text: "after", OldNumber: 4, NewNumber: 3}}}}}, {DisplayPath: "second.go", OldPath: "second.go", NewPath: "second.go", Hunks: []patch.Hunk{{Header: "@@ -1 +1 @@", Lines: []patch.Line{{Kind: patch.Addition, Text: "other", NewNumber: 1}}}}}}}, Options{})
	v.Resize(80, 20)
	v.BeginDrag(10, 3)
	v.DragTo(10, 4)
	cursor, selection := v.cursor, *v.selection
	v.DragTo(10, 10)
	testutil.RequireEqual(t, v.cursor, cursor, "%s",
		"drag crossed file/hunk boundary",
	)
	testutil.RequireEqual(t, *v.selection, selection, "%s",
		"drag crossed file/hunk boundary",
	)

	for _, change := range []func(){
		func() {
			v.Update(patch.Patch{Root: "/repo", Files: []patch.File{{DisplayPath: "first.go", OldPath: "first.go", NewPath: "first.go", Hunks: []patch.Hunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "before", OldNumber: 1, NewNumber: 1}, {Kind: patch.Deletion, Text: "removed one", OldNumber: 2}, {Kind: patch.Deletion, Text: "removed two", OldNumber: 3}, {Kind: patch.Addition, Text: "added one", NewNumber: 2}, {Kind: patch.Context, Text: "after", OldNumber: 4, NewNumber: 3}}}}}, {DisplayPath: "second.go", OldPath: "second.go", NewPath: "second.go", Hunks: []patch.Hunk{{Header: "@@ -1 +1 @@", Lines: []patch.Line{{Kind: patch.Addition, Text: "other", NewNumber: 1}}}}}}})
		},
		func() { v.Resize(80, 20) },
		func() { v.Move(NextLine) },
		func() { v.ClearSelection() },
	} {
		v.BeginDrag(10, 3)
		change()
		testutil.RequireEqual(t, v.drag, nil, "%s",
			"stale drag survived state change",
		)
	}
}
