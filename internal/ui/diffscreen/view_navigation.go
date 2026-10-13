package diffscreen

// Move performs navigation and keeps the resulting focus visible. Line and
// page movement extend active selection only within its original hunk.
func (v *View) Move(motion Motion) {
	v.EndDrag()
	v.search.repeatMiss = false
	switch motion {
	case PreviousLine:
		v.step(backward)
	case NextLine:
		v.step(forward)
	case PreviousPage:
		v.halfPage(backward)
	case NextPage:
		v.halfPage(forward)
	case FirstLine:
		if cursor, ok := v.projection.firstCursor(); ok {
			v.setCursor(cursor)
		}
	case LastLine:
		if cursor, ok := v.projection.lastCursor(); ok {
			v.setCursor(cursor)
		}

	default:
		v.moveBetweenFilesAndPanes(motion)
	}
}

func (v *View) moveBetweenFilesAndPanes(motion Motion) {
	switch motion {
	case PreviousFile:
		v.jumpToAdjacentFile(backward)
	case NextFile:
		v.jumpToAdjacentFile(forward)
	case OldPane:
		v.switchCursorPane(left)
	case NewPane:
		v.switchCursorPane(right)
	case OtherPane:
		v.switchCursorPane(v.cursor.pane.other())
	default:
		return
	}
}

func (v *View) step(direction direction) {
	next, ok := v.projection.moveCursor(v.cursor, direction)
	if !ok {
		return
	}
	if !v.extendRangeTo(next) {
		return
	}
	v.setCursor(next)
}

func (v *View) setCursor(cursor patchCursor) {
	v.cursor = cursor
	v.viewport = v.projection.keepCursorVisible(v.viewport, cursor)
}

func (v *View) halfPage(direction direction) {
	viewport, cursor := v.projection.scrollHalfPage(
		v.viewport, v.cursor, direction,
	)
	if !v.extendRangeTo(cursor) {
		return
	}
	v.viewport, v.cursor = viewport, cursor
}

// extendRangeTo accepts unrestricted movement when no range is active.
// An active range changes only when its endpoint remains in the original hunk.
func (v *View) extendRangeTo(cursor patchCursor) bool {
	if v.selection == nil {
		return true
	}
	selection, ok := v.projection.extendRange(*v.selection, cursor)
	if ok {
		v.selection = &selection
	}
	return ok
}

func (v *View) jumpToAdjacentFile(direction direction) {
	v.ClearSelection()
	if cursor, ok := v.projection.jumpToAdjacentFile(v.cursor, direction); ok {
		v.setCursor(cursor)
	}
}

func (v *View) switchCursorPane(pane patchPane) {
	cursor, ok := v.projection.switchCursorPane(v.cursor, pane)
	if !ok {
		return
	}
	if v.selection != nil {
		firstCursor, firstOK := v.projection.switchCursorPane(
			v.selection.First,
			pane,
		)
		lastCursor, lastOK := v.projection.switchCursorPane(
			v.selection.Last,
			pane,
		)
		if !firstOK || !lastOK {
			return
		}
		selection := beginRange(firstCursor)
		selection, ok = v.projection.extendRange(selection, lastCursor)
		if !ok {
			return
		}
		v.selection = &selection
	}
	v.setCursor(cursor)
}

// Align positions the focused line at the requested viewport alignment.
func (v *View) Align(alignment Alignment) {
	v.search.repeatMiss = false
	if _, ok := v.projection.lineAtCursor(v.cursor); !ok {
		return
	}
	v.viewport = v.projection.alignCursor(
		v.viewport,
		v.cursor,
		verticalAlignment(alignment),
	)
}

// ScrollHorizontal shifts the visible content by the requested columns.
func (v *View) ScrollHorizontal(columns int) {
	v.search.repeatMiss = false
	v.viewport = v.projection.scrollHorizontal(v.viewport, columns)
}

// ScrollVertical moves content first, moving focus only when it leaves the
// visible code. Like keyboard movement, it extends selection within its hunk.
func (v *View) ScrollVertical(rows int) {
	v.search.repeatMiss = false
	viewport, cursor := v.projection.scrollVertical(v.viewport, v.cursor, rows)
	if cursor != v.cursor && !v.extendRangeTo(cursor) {
		return
	}
	v.viewport, v.cursor = viewport, cursor
}
