package ui

import (
	"testing"
)

func TestDiffscreenClickAndDragSelectCode(t *testing.T) {
	v := newDiffscreenTestView(
		diffscreenTwoFileSelectionPatch(), Options{},
	)
	v.Resize(80, 20)
	requireEqual(t, v.BeginDrag(20, 4), true, "%s", "code click was ignored")

	_, line, _ := v.Current()
	requireEqual(t, line.Text, "removed one", "%s",
		"click did not focus code without selecting a range",
	)
	requireEqual(t, v.selection, nil, "%s",
		"click did not focus code without selecting a range",
	)

	v.DragTo(30, 6)
	v.EndDrag()
	_, lines, ok := v.Selected()
	requireEqual(t, ok, true, "drag selection = %#v", lines)
	requireEqual(t, len(lines), 3, "drag selection = %#v", lines)
	requireEqual(t, lines[0].Text, "removed one", "drag selection = %#v", lines)
	requireEqual(t, lines[2].Text, "added one", "drag selection = %#v", lines)

	end := v.cursor
	v.DragTo(20, 3)
	requireEqual(t, v.cursor, end, "%s",
		"motion after release changed selection",
	)
	requireEqual(t, v.BeginDrag(20, 6), true, "%s",
		"new click did not clear old selection",
	)
	requireEqual(t, v.selection, nil, "%s",
		"new click did not clear old selection",
	)

	v.DragTo(20, 3)
	_, lines, ok = v.Selected()
	requireEqual(t, ok, true, "backward drag = %#v", lines)
	requireEqual(t, len(lines), 4, "backward drag = %#v", lines)

	v.DragTo(20, 6)
	_, lines, _ = v.Selected()
	requireEqual(t, len(lines), 1, "%s",
		"drag back to origin did not shrink selection",
	)
}

func TestDiffscreenMouseHitTestingMatchesScrolledProjection(t *testing.T) {
	for _, split := range []bool{false, true} {
		v := newDiffscreenTestView(
			diffscreenLongContextPatch(),
			Options{SideBySide: split},
		)
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
			requireEqual(t, !v.BeginDrag(point[0], point[1]), true,
				"non-code point %v accepted", point,
			)
		}
		requireEqual(t, v.BeginDrag(100, 2), true, "%s",
			"click did not account for viewport and sticky header",
		)
		requireEqual(t, v.cursor.row, 5, "%s",
			"click did not account for viewport and sticky header",
		)
		requireEqual(t, v.cursor.pane, right, "%s",
			"click did not account for viewport and sticky header",
		)

		v.DragTo(100, 4)
		_, lines, ok := v.Selected()
		requireEqual(t, ok, true, "scrolled drag selection = %#v", lines)
		requireEqual(t, len(lines), 3, "scrolled drag selection = %#v", lines)
	}
}

func TestDiffscreenSplitMouseRejectsDividerEmptyPaneAndCrossPaneDrag(
	t *testing.T,
) {
	v := newDiffscreenTestView(
		diffscreenTwoFileSelectionPatch(),
		Options{SideBySide: true},
	)
	v.Resize(120, 20)
	for _, x := range []int{58, 59, 60} {
		requireEqual(t, !v.BeginDrag(x, 3), true, "%s", "divider accepted")
	}
	requireEqual(t, !v.BeginDrag(80, 5), true, "%s",
		"empty right pane accepted",
	)
	requireEqual(t, v.BeginDrag(10, 4), true, "%s",
		"deletion click did not focus old pane",
	)
	requireEqual(t, v.cursor.pane, left, "%s",
		"deletion click did not focus old pane",
	)

	origin := v.cursor
	v.DragTo(80, 4)
	requireEqual(t, v.cursor, origin, "%s", "drag crossed panes")
	requireEqual(t, v.selection, nil, "%s", "drag crossed panes")

	v.DragTo(10, 5)
	_, lines, ok := v.Selected()
	requireEqual(t, ok, true, "old-pane drag = %#v", lines)
	requireEqual(t, len(lines), 2, "old-pane drag = %#v", lines)
	requireEqual(t, lines[1].Text, "removed two", "old-pane drag = %#v", lines)
}

func TestDiffscreenDragRejectsOtherHunksAndCancelsOnStateChanges(t *testing.T) {
	v := newDiffscreenTestView(
		diffscreenTwoFileSelectionPatch(), Options{},
	)
	v.Resize(80, 20)
	v.BeginDrag(10, 3)
	v.DragTo(10, 4)
	cursor, selection := v.cursor, *v.selection
	v.DragTo(10, 10)
	requireEqual(t, v.cursor, cursor, "%s", "drag crossed file/hunk boundary")
	requireEqual(t, *v.selection, selection, "%s",
		"drag crossed file/hunk boundary",
	)

	for _, change := range []func(){
		func() { v.Update(diffscreenTwoFileSelectionPatch()) },
		func() { v.Resize(80, 20) },
		func() { v.Move(NextLine) },
		func() { v.ClearSelection() },
	} {
		v.BeginDrag(10, 3)
		change()
		requireEqual(t, v.drag, nil, "%s", "stale drag survived state change")
	}
}
