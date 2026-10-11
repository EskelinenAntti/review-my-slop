package diffscreen

import (
	"testing"
)

func TestClickAndDragSelectCode(t *testing.T) {
	v := New(testPatch(), Options{})
	v.Resize(defaultWidth, minimumWidth)
	requireEqual(
		t,
		v.BeginDrag(minimumWidth, 4),
		true,
		fixtureV,
		"code click was ignored",
	)
	_, line, _ := v.Current()
	requireEqual(
		t,
		line.Text == "removed one" && v.selection == nil,
		true,
		fixtureV,
		"click did not focus code without selecting a range",
	)
	v.DragTo(defaultHeight, 6)
	v.EndDrag()
	_, lines, ok := v.Selected()
	requireEqual(
		t,
		ok && len(lines) == 3 && lines[0].Text == "removed one" &&
			lines[2].Text == "added one",
		true,
		"drag selection = %#v",
		lines,
	)
	end := v.cursor
	v.DragTo(minimumWidth, 3)
	requireEqual(
		t,
		v.cursor, end,
		fixtureV,
		"motion after release changed selection",
	)
	clicked := v.BeginDrag(minimumWidth, 6)
	requireEqual(
		t,
		clicked && v.selection == nil,
		true,
		fixtureV,
		"new click did not clear old selection",
	)
	v.DragTo(minimumWidth, 3)
	_, lines, ok = v.Selected()
	requireEqual(
		t,
		ok && len(lines) == 4, true,
		"backward drag = %#v",
		lines,
	)
	v.DragTo(minimumWidth, 6)
	_, lines, _ = v.Selected()
	requireEqual(
		t,
		len(lines), 1,
		fixtureV,
		"drag back to origin did not shrink selection",
	)
}

func TestMouseHitTestingMatchesScrolledProjection(t *testing.T) {
	for _, split := range []bool{false, true} {
		v := New(longPatch(), Options{SideBySide: split})
		v.Resize(oddTestWidth, 10)
		v.ScrollVertical(5)
		v.ScrollHorizontal(minimumWidth)
		for _, point := range [][2]int{
			{0, 0},
			{0, 1},
			{0, 8},
			{0, 9},
			{-1, 3},
			{oddTestWidth, 3},
		} {
			requireEqual(
				t,
				!v.BeginDrag(point[0], point[1]), true,
				"non-code point %v accepted",
				point,
			)
		}
		clicked := v.BeginDrag(100, 2)
		requireEqual(
			t,
			clicked && v.cursor.row == 5 && v.cursor.pane == right,
			true,
			fixtureV,
			"click did not account for viewport and sticky header",
		)
		v.DragTo(100, 4)
		_, lines, ok := v.Selected()
		requireEqual(
			t,
			ok && len(lines) == 3, true,
			"scrolled drag selection = %#v",
			lines,
		)
	}
}

func TestSplitMouseRejectsDividerEmptyPaneAndCrossPaneDrag(t *testing.T) {
	v := New(testPatch(), Options{SideBySide: true})
	v.Resize(wideTestWidth, minimumWidth)
	dividerStart, _ := splitPaneWidths(wideTestWidth)
	for x := dividerStart; x < dividerStart+paneDividerWidth; x++ {
		requireEqual(
			t,
			!v.BeginDrag(x, 3),
			true,
			fixtureV,
			"divider accepted",
		)
	}
	requireEqual(
		t,
		!v.BeginDrag(defaultWidth, 5),
		true,
		fixtureV,
		"empty right pane accepted",
	)
	clicked := v.BeginDrag(10, 4)
	requireEqual(
		t,
		clicked && v.cursor.pane == left,
		true,
		fixtureV,
		"deletion click did not focus old pane",
	)
	origin := v.cursor
	v.DragTo(defaultWidth, 4)
	requireEqual(
		t,
		v.cursor == origin && v.selection == nil,
		true,
		fixtureV,
		"drag crossed panes",
	)
	v.DragTo(10, 5)
	_, lines, ok := v.Selected()
	requireEqual(
		t,
		ok && len(lines) == 2 && lines[1].Text == "removed two", true,
		"old-pane drag = %#v",
		lines,
	)
}

func TestDragRejectsOtherHunksAndCancelsOnStateChanges(t *testing.T) {
	v := New(testPatch(), Options{})
	v.Resize(defaultWidth, minimumWidth)
	v.BeginDrag(10, 3)
	v.DragTo(10, 4)
	cursor, selection := v.cursor, *v.selection
	v.DragTo(10, 10)
	requireEqual(
		t,
		v.cursor == cursor && *v.selection == selection,
		true,
		fixtureV,
		"drag crossed file/hunk boundary",
	)
	for _, change := range []func(){
		func() { v.Update(testPatch()) },
		func() { v.Resize(defaultWidth, minimumWidth) },
		func() { v.Move(NextLine) },
		func() { v.ClearSelection() },
	} {
		v.BeginDrag(10, 3)
		change()
		requireEqual(
			t,
			v.drag == nil,
			true,
			fixtureV,
			"stale drag survived state change",
		)
	}
}
