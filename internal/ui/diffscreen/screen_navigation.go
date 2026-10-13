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
		if cursor, ok := v.view.first(); ok {
			v.setCursor(cursor)
		}
	case LastLine:
		if cursor, ok := v.view.last(); ok {
			v.setCursor(cursor)
		}

	default:
		v.moveBetweenSections(motion)
	}
}

func (v *View) moveBetweenSections(motion Motion) {
	switch motion {
	case PreviousFile:
		v.jumpFile(backward)
	case NextFile:
		v.jumpFile(forward)
	case OldPane:
		v.switchPane(left)
	case NewPane:
		v.switchPane(right)
	case OtherPane:
		v.switchPane(v.cursor.pane.other())
	default:
		return
	}
}

func (v *View) step(direction direction) {
	next, ok := v.view.move(v.cursor, direction)
	if !ok {
		return
	}
	if !v.extendSelectionTo(next) {
		return
	}
	v.setCursor(next)
}

func (v *View) setCursor(cursor diffCursor) {
	v.cursor = cursor
	v.viewport = v.view.keepVisible(v.viewport, cursor)
}

func (v *View) halfPage(direction direction) {
	viewport, cursor := v.view.scrollHalfPage(v.viewport, v.cursor, direction)
	if !v.extendSelectionTo(cursor) {
		return
	}
	v.viewport, v.cursor = viewport, cursor
}

// extendSelectionTo accepts unrestricted movement when no range is active.
// An active range changes only when its endpoint remains in the original hunk.
func (v *View) extendSelectionTo(cursor diffCursor) bool {
	if v.selection == nil {
		return true
	}
	selection, ok := v.view.extendSelection(*v.selection, cursor)
	if ok {
		v.selection = &selection
	}
	return ok
}

func (v *View) jumpFile(direction direction) {
	v.ClearSelection()
	if cursor, ok := v.view.jumpFile(v.cursor, direction); ok {
		v.setCursor(cursor)
	}
}

func (v *View) switchPane(pane diffPane) {
	cursor, ok := v.view.switchPane(v.cursor, pane)
	if !ok {
		return
	}
	if v.selection != nil {
		first, firstOK := v.view.switchPane(v.selection.First, pane)
		last, lastOK := v.view.switchPane(v.selection.Last, pane)
		if !firstOK || !lastOK {
			return
		}
		selection := beginSelection(first)
		selection, ok = v.view.extendSelection(selection, last)
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
	if _, ok := v.view.line(v.cursor); !ok {
		return
	}
	v.viewport = v.view.align(
		v.viewport,
		v.cursor,
		verticalAlignment(alignment),
	)
}

// ScrollHorizontal shifts the visible content by the requested columns.
func (v *View) ScrollHorizontal(columns int) {
	v.search.repeatMiss = false
	v.viewport = v.view.scrollHorizontal(v.viewport, columns)
}

// ScrollVertical moves content first, moving focus only when it leaves the
// visible code. Like keyboard movement, it extends selection within its hunk.
func (v *View) ScrollVertical(rows int) {
	v.search.repeatMiss = false
	viewport, cursor := v.view.scrollVertical(v.viewport, v.cursor, rows)
	if cursor != v.cursor && !v.extendSelectionTo(cursor) {
		return
	}
	v.viewport, v.cursor = viewport, cursor
}
