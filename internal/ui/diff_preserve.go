package ui

type cursorIdentity struct {
	file   reviewFile
	hunk   reviewHunk
	line   reviewLine
	cursor Cursor
	valid  bool
}

// Preserve returns fresh State for next by retaining the meaningful position
// from state where next contains it.
func Preserve(old *diffView, state State, next *diffView) State {
	viewport, oldCursor := state.Viewport, state.Cursor
	result := State{Viewport: next.Resize(Viewport{}, viewport.Width, viewport.Height)}
	rowsAbove := 0
	if oldCursor != nil {
		rowsAbove = oldCursor.Coordinate - viewport.Top
	}

	cursor := identify(old, oldCursor)
	var translated *Cursor
	if cursor.valid {
		if candidate, ok := next.FindCursor(cursor.file, cursor.hunk, cursor.line, cursor.cursor.Coordinate, cursor.cursor.Pane); ok {
			translated = &candidate
		}
	}
	if translated == nil {
		if first, ok := next.First(); ok {
			translated = &first
		}
	}

	if selection, ok := preserveSelection(old, state.Selection, next); ok {
		result.Selection = &selection
	}
	result.Cursor = translated
	if translated != nil {
		result.Viewport.Top = max(0, translated.Coordinate-rowsAbove)
		result.Viewport = next.KeepVisible(result.Viewport, *translated)
	}
	return result
}

func identify(v *diffView, cursor *Cursor) cursorIdentity {
	if cursor != nil && v.valid(*cursor) {
		current := v.rows[cursor.Coordinate]
		file := v.patch.Files[current.file]
		hunk := file.Hunks[current.hunk]
		line := hunk.Lines[v.lineIndex(current, cursor.Pane)]
		return cursorIdentity{file, hunk, line, *cursor, true}
	}
	return cursorIdentity{}
}

func preserveSelection(old *diffView, selection *Selection, next *diffView) (Selection, bool) {
	if selection == nil {
		return Selection{}, false
	}
	first, last := identify(old, &selection.First), identify(old, &selection.Last)
	if first.valid && last.valid {
		translatedFirst, firstOK := next.FindCursor(first.file, first.hunk, first.line, first.cursor.Coordinate, first.cursor.Pane)
		translatedLast, lastOK := next.FindCursor(last.file, last.hunk, last.line, last.cursor.Coordinate, last.cursor.Pane)
		if firstOK && lastOK && sameFile(first.file, last.file) && first.hunk.Header == last.hunk.Header {
			return next.ExtendSelection(Selection{translatedFirst, translatedFirst}, translatedLast)
		}
	}
	return Selection{}, false
}
