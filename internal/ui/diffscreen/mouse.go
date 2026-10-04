package diffscreen

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
	if !ok || cursor.pane != v.drag.pane {
		return
	}
	if cursor == *v.drag && v.selection == nil {
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
	if x < 0 || x >= v.width || y < 1 || y >= 1+v.viewport.Height || y >= v.height-2 {
		return diffCursor{}, false
	}
	row := y - 1
	if v.view.hasStickyHeader(v.viewport.top, v.viewport.Height) {
		row--
	}
	if row < 0 || row >= v.view.contentHeight(v.viewport) {
		return diffCursor{}, false
	}
	pane := right
	if v.view.split {
		leftWidth := max(20, (v.viewport.Width-3)/2)
		switch {
		case x < leftWidth:
			pane = left
		case x < leftWidth+3:
			return diffCursor{}, false
		}
	}
	return v.view.cursorAt(v.viewport.top+row, pane)
}
