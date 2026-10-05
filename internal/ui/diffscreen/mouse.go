package diffscreen

import "github.com/eskelinenantti/review-my-slop/internal/ui/internal/frame"

// BeginDrag focuses the code under a left-button press and clears any previous
// selection. Coordinates are zero-based terminal cells, including the header.
// Headers, metadata, empty panes, and cells outside the body are ignored.
func (v *View) BeginDrag(x, y int) bool {
	v.EndDrag()
	cursor, ok := v.cursorAtPoint(x, y)
	if !ok {
		return false
	}
	v.ClearSelection()
	v.cursor = cursor
	v.drag = &cursor
	return true
}

// DragTo extends a mouse selection within the starting hunk and pane. It does
// nothing without a code-line press, or when the pointer is over invalid code.
func (v *View) DragTo(x, y int) {
	if v.drag == nil {
		return
	}
	cursor, ok := v.cursorAtPoint(x, y)
	if !ok || cursor.pane != v.drag.pane || cursor == *v.drag && v.selection == nil {
		return
	}
	selection, ok := v.view.extendSelection(v.view.beginSelection(*v.drag), cursor)
	if !ok {
		return
	}
	v.cursor, v.selection = cursor, &selection
}

// EndDrag leaves a completed selection available for keyboard actions.
func (v *View) EndDrag() { v.drag = nil }

func (v *View) cursorAtPoint(x, y int) (diffCursor, bool) {
	row, ok := frame.BodyRow(x, y, v.width, v.height)
	if v.view.hasStickyHeader(v.viewport.top, v.viewport.Height) {
		row--
	}
	if !ok || row < 0 || row >= v.view.contentHeight(v.viewport) {
		return diffCursor{}, false
	}
	pane := right
	if v.view.split {
		leftWidth, _ := splitPaneWidths(v.viewport.Width)
		switch {
		case x < leftWidth:
			pane = left
		case x < leftWidth+3:
			return diffCursor{}, false
		}
	}
	return v.view.cursorAt(v.viewport.top+row, pane)
}
