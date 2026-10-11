package diffscreen

import (
	"testing"
)

func TestClickAndDragSelectCode(t *testing.T) {
	v := New(testPatch(), Options{})
	v.Resize(defaultWidth, minimumWidth)
	requireEqual(
		t,
		v.BeginDrag(minimumWidth, four),
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
	v.DragTo(defaultHeight, six)
	v.EndDrag()
	_, lines, ok := v.Selected()
	requireEqual(
		t,
		ok && len(lines) == three && lines[zero].Text == "removed one" &&
			lines[two].Text == "added one",
		true,
		"drag selection = %#v",
		lines,
	)
	end := v.cursor
	v.DragTo(minimumWidth, three)
	requireEqual(
		t,
		v.cursor, end,
		fixtureV,
		"motion after release changed selection",
	)
	clicked := v.BeginDrag(minimumWidth, six)
	requireEqual(
		t,
		clicked && v.selection == nil,
		true,
		fixtureV,
		"new click did not clear old selection",
	)
	v.DragTo(minimumWidth, three)
	_, lines, ok = v.Selected()
	requireEqual(
		t,
		ok && len(lines) == four, true,
		"backward drag = %#v",
		lines,
	)
	v.DragTo(minimumWidth, six)
	_, lines, _ = v.Selected()
	requireEqual(
		t,
		len(lines), one,
		fixtureV,
		"drag back to origin did not shrink selection",
	)
}

func TestMouseHitTestingMatchesScrolledProjection(t *testing.T) {
	for _, split := range []bool{false, true} {
		v := New(longPatch(), Options{SideBySide: split})
		v.Resize(oddTestWidth, ten)
		v.ScrollVertical(five)
		v.ScrollHorizontal(minimumWidth)
		for _, point := range [][two]int{
			{zero, zero},
			{zero, one},
			{zero, eight},
			{zero, nine},
			{-one, three},
			{oddTestWidth, three},
		} {
			requireEqual(
				t,
				!v.BeginDrag(point[zero], point[one]), true,
				"non-code point %v accepted",
				point,
			)
		}
		clicked := v.BeginDrag(hundred, two)
		requireEqual(
			t,
			clicked && v.cursor.row == five && v.cursor.pane == right,
			true,
			fixtureV,
			"click did not account for viewport and sticky header",
		)
		v.DragTo(hundred, four)
		_, lines, ok := v.Selected()
		requireEqual(
			t,
			ok && len(lines) == three, true,
			"scrolled drag selection = %#v",
			lines,
		)
	}
}

func TestSplitMouseRejectsDividerEmptyPaneAndCrossPaneDrag(t *testing.T) {
	v := New(testPatch(), Options{SideBySide: true})
	v.Resize(wideTestWidth, minimumWidth)
	for _, x := range []int{fixtureNumber58, fixtureNumber59, sixty} {
		requireEqual(
			t,
			!v.BeginDrag(x, three),
			true,
			fixtureV,
			"divider accepted",
		)
	}
	requireEqual(
		t,
		!v.BeginDrag(defaultWidth, five),
		true,
		fixtureV,
		"empty right pane accepted",
	)
	clicked := v.BeginDrag(ten, four)
	requireEqual(
		t,
		clicked && v.cursor.pane == left,
		true,
		fixtureV,
		"deletion click did not focus old pane",
	)
	origin := v.cursor
	v.DragTo(defaultWidth, four)
	requireEqual(
		t,
		v.cursor == origin && v.selection == nil,
		true,
		fixtureV,
		"drag crossed panes",
	)
	v.DragTo(ten, five)
	_, lines, ok := v.Selected()
	requireEqual(
		t,
		ok && len(lines) == two && lines[one].Text == "removed two", true,
		"old-pane drag = %#v",
		lines,
	)
}

func TestDragRejectsOtherHunksAndCancelsOnStateChanges(t *testing.T) {
	v := New(testPatch(), Options{})
	v.Resize(defaultWidth, minimumWidth)
	v.BeginDrag(ten, three)
	v.DragTo(ten, four)
	cursor, selection := v.cursor, *v.selection
	v.DragTo(ten, ten)
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
		v.BeginDrag(ten, three)
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
