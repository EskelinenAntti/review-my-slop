package diffscreen

import (
	"fmt"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestClickAndDragSelectCode(t *testing.T) {
	v := New(twoFileReviewPatch(), Options{})
	v.Resize(80, 20)
	testutil.RequireEqual(t, v.BeginDrag(20, 4), true,
		"code click was ignored",
	)

	_, line, _ := v.Current()
	testutil.RequireEqual(t, line.Text, "removed one",
		"click did not focus code without selecting a range",
	)
	testutil.RequireEqual(t, v.selection, nil,
		"click did not focus code without selecting a range",
	)

	v.DragTo(30, 6)
	v.EndDrag()
	_, lines, ok := v.Selected()
	testutil.RequireEqual(t, ok, true,
		fmt.Sprintf("drag selection = %#v", lines),
	)
	testutil.RequireEqual(t, len(lines), 3,
		fmt.Sprintf("drag selection = %#v", lines),
	)
	testutil.RequireEqual(t, lines[0].Text, "removed one",
		fmt.Sprintf("drag selection = %#v", lines),
	)
	testutil.RequireEqual(t, lines[2].Text, "added one",
		fmt.Sprintf("drag selection = %#v", lines),
	)

	end := v.cursor
	v.DragTo(20, 3)
	testutil.RequireEqual(t, v.cursor, end,
		"motion after release changed selection",
	)
	testutil.RequireEqual(t, v.BeginDrag(20, 6), true,
		"new click did not clear old selection",
	)
	testutil.RequireEqual(t, v.selection, nil,
		"new click did not clear old selection",
	)

	v.DragTo(20, 3)
	_, lines, ok = v.Selected()
	testutil.RequireEqual(t, ok, true,
		fmt.Sprintf("backward drag = %#v", lines),
	)
	testutil.RequireEqual(t, len(lines), 4,
		fmt.Sprintf("backward drag = %#v", lines),
	)

	v.DragTo(20, 6)
	_, lines, _ = v.Selected()
	testutil.RequireEqual(t, len(lines), 1,
		"drag back to origin did not shrink selection",
	)
}

func TestMouseHitTestingMatchesScrolledProjection(t *testing.T) {
	for _, split := range []bool{false, true} {
		v := New(manyContextLinesPatch(), Options{SideBySide: split})
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
				fmt.Sprintf("non-code point %v accepted", point),
			)
		}
		testutil.RequireEqual(t, v.BeginDrag(100, 2), true,
			"click did not account for viewport and sticky header",
		)
		testutil.RequireEqual(t, v.cursor.row, 5,
			"click did not account for viewport and sticky header",
		)
		testutil.RequireEqual(t, v.cursor.pane, right,
			"click did not account for viewport and sticky header",
		)

		v.DragTo(100, 4)
		_, lines, ok := v.Selected()
		testutil.RequireEqual(t, ok, true,
			fmt.Sprintf("scrolled drag selection = %#v", lines),
		)
		testutil.RequireEqual(t, len(lines), 3,
			fmt.Sprintf("scrolled drag selection = %#v", lines),
		)
	}
}

func TestSplitMouseRejectsDividerEmptyPaneAndCrossPaneDrag(t *testing.T) {
	v := New(twoFileReviewPatch(), Options{SideBySide: true})
	v.Resize(120, 20)
	for _, x := range []int{58, 59, 60} {
		testutil.RequireEqual(t, !v.BeginDrag(x, 3), true, "divider accepted")
	}
	testutil.RequireEqual(t, !v.BeginDrag(80, 5), true,
		"empty right pane accepted",
	)
	testutil.RequireEqual(t, v.BeginDrag(10, 4), true,
		"deletion click did not focus old pane",
	)
	testutil.RequireEqual(t, v.cursor.pane, left,
		"deletion click did not focus old pane",
	)

	origin := v.cursor
	v.DragTo(80, 4)
	testutil.RequireEqual(t, v.cursor, origin, "drag crossed panes")
	testutil.RequireEqual(t, v.selection, nil, "drag crossed panes")

	v.DragTo(10, 5)
	_, lines, ok := v.Selected()
	testutil.RequireEqual(t, ok, true,
		fmt.Sprintf("old-pane drag = %#v", lines),
	)
	testutil.RequireEqual(t, len(lines), 2,
		fmt.Sprintf("old-pane drag = %#v", lines),
	)
	testutil.RequireEqual(t, lines[1].Text, "removed two",
		fmt.Sprintf("old-pane drag = %#v", lines),
	)
}

func TestDragRejectsOtherHunksAndCancelsOnStateChanges(t *testing.T) {
	v := New(twoFileReviewPatch(), Options{})
	v.Resize(80, 20)
	v.BeginDrag(10, 3)
	v.DragTo(10, 4)
	cursor, selection := v.cursor, *v.selection
	v.DragTo(10, 10)
	testutil.RequireEqual(t, v.cursor, cursor,
		"drag crossed file/hunk boundary",
	)
	testutil.RequireEqual(t, *v.selection, selection,
		"drag crossed file/hunk boundary",
	)

	for _, change := range []func(){
		func() { v.Update(twoFileReviewPatch()) },
		func() { v.Resize(80, 20) },
		func() { v.Move(NextLine) },
		func() { v.ClearSelection() },
	} {
		v.BeginDrag(10, 3)
		change()
		testutil.RequireEqual(t, v.drag, nil,
			"stale drag survived state change",
		)
	}
}
