package diffscreen

import (
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestClickAndDragSelectCode(t *testing.T) {
	v := New(testPatch(), Options{})
	v.Resize(80, 20)
	testutil.Require(t, v.BeginDrag(20, 4), "code click was ignored")
	_, line, _ := v.Current()
	testutil.Require(t, line.Text == "removed one" && v.selection == nil, "click did not focus code without selecting a range")
	v.DragTo(30, 6)
	v.EndDrag()
	_, lines, ok := v.Selected()
	testutil.Requiref(t, ok && len(lines) == 3 && lines[0].Text == "removed one" && lines[2].Text == "added one", "drag selection = %#v", lines)
	end := v.cursor
	v.DragTo(20, 3)
	testutil.Require(t, v.cursor == end, "motion after release changed selection")
	testutil.Require(t, v.BeginDrag(20, 6) && v.selection == nil, "new click did not clear old selection")
	v.DragTo(20, 3)
	_, lines, ok = v.Selected()
	testutil.Requiref(t, ok && len(lines) == 4, "backward drag = %#v", lines)
	v.DragTo(20, 6)
	_, lines, _ = v.Selected()
	testutil.Require(t, len(lines) == 1, "drag back to origin did not shrink selection")
}

func TestMouseHitTestingMatchesScrolledProjection(t *testing.T) {
	for _, split := range []bool{false, true} {
		v := New(longPatch(), Options{SideBySide: split})
		v.Resize(121, 10)
		v.ScrollVertical(5)
		v.ScrollHorizontal(20)
		assertInvalidMousePoints(t, v)
		testutil.Require(t, v.BeginDrag(100, 2) && v.cursor.row == 5 && v.cursor.pane == right, "click did not account for viewport and sticky header")
		v.DragTo(100, 4)
		_, lines, ok := v.Selected()
		testutil.Requiref(t, ok && len(lines) == 3, "scrolled drag selection = %#v", lines)
	}
}

func TestSplitMouseRejectsDividerEmptyPaneAndCrossPaneDrag(t *testing.T) {
	v := New(testPatch(), Options{SideBySide: true})
	v.Resize(120, 20)
	for _, x := range []int{58, 59, 60} {
		testutil.Require(t, !v.BeginDrag(x, 3), "divider accepted")
	}
	testutil.Require(t, !v.BeginDrag(80, 5), "empty right pane accepted")
	testutil.Require(t, v.BeginDrag(10, 4) && v.cursor.pane == left, "deletion click did not focus old pane")
	origin := v.cursor
	v.DragTo(80, 4)
	testutil.Require(t, v.cursor == origin && v.selection == nil, "drag crossed panes")
	v.DragTo(10, 5)
	_, lines, ok := v.Selected()
	testutil.Requiref(t, ok && len(lines) == 2 && lines[1].Text == "removed two", "old-pane drag = %#v", lines)
}

func TestDragRejectsOtherHunksAndCancelsOnStateChanges(t *testing.T) {
	v := New(testPatch(), Options{})
	v.Resize(80, 20)
	v.BeginDrag(10, 3)
	v.DragTo(10, 4)
	cursor, selection := v.cursor, *v.selection
	v.DragTo(10, 10)
	testutil.Require(t, v.cursor == cursor && *v.selection == selection, "drag crossed file/hunk boundary")
	for _, change := range []func(){
		func() { v.Update(testPatch()) },
		func() { v.Resize(80, 20) },
		func() { v.Move(NextLine) },
		func() { v.ClearSelection() },
	} {
		v.BeginDrag(10, 3)
		change()
		testutil.Require(t, v.drag == nil, "stale drag survived state change")
	}
}

func assertInvalidMousePoints(t *testing.T, v *View) {
	t.Helper()
	for _, point := range [][2]int{{0, 0}, {0, 1}, {0, 8}, {0, 9}, {-1, 3}, {121, 3}} {
		testutil.Requiref(t, !v.BeginDrag(point[0], point[1]), "non-code point %v accepted", point)
	}
}
