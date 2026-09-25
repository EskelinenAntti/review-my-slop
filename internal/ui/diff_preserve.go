package ui

type cursorIdentity struct {
	file   diffFile
	hunk   diffHunk
	line   diffLine
	cursor Cursor
	valid  bool
}

// Preserve returns fresh State for next by retaining the meaningful position
// from state where next contains it.
func Preserve(old View, state State, next View) State {
	oldCursor, oldViewport := state.Cursor, state.Viewport
	viewport := next.NewViewport(oldViewport.Width, oldViewport.Height)
	result := State{Viewport: viewport}
	rowsAbove := 0
	if oldCursor != nil {
		rowsAbove = oldCursor.Coordinate.Y - oldViewport.Top.Y
	}

	identity := identify(old, oldCursor)
	var cursor *Cursor
	if translated, ok := translateCursor(next, identity); ok {
		cursor = &translated
	}
	if cursor == nil {
		if first, ok := next.First(); ok {
			cursor = &first
		}
	}

	if selection, ok := preserveSelection(old, state.Selection, next); ok {
		result.Selection = &selection
	}
	if cursor != nil {
		viewport.Top.Y = max(0, cursor.Coordinate.Y-rowsAbove)
		viewport = next.KeepVisible(viewport, *cursor)
	}
	result.Cursor, result.Viewport = cursor, viewport
	return result
}

func identify(v View, cursor *Cursor) cursorIdentity {
	if cursor == nil {
		return cursorIdentity{}
	}
	file, fileOK := v.File(*cursor)
	hunk, hunkOK := v.Hunk(*cursor)
	line, lineOK := v.Line(*cursor)
	return cursorIdentity{file: file, hunk: hunk, line: line, cursor: *cursor, valid: fileOK && hunkOK && lineOK}
}

func translateCursor(v View, identity cursorIdentity) (Cursor, bool) {
	if !identity.valid {
		return Cursor{}, false
	}
	cursor := identity.cursor
	return v.FindCursor(identity.file, identity.hunk, identity.line, cursor.Coordinate, cursor.Pane)
}

func preserveSelection(old View, selection *Selection, next View) (Selection, bool) {
	if selection == nil {
		return Selection{}, false
	}
	first := identify(old, &selection.First)
	last := identify(old, &selection.Last)
	translatedFirst, firstOK := translateCursor(next, first)
	translatedLast, lastOK := translateCursor(next, last)
	if !firstOK || !lastOK || !sameFile(first.file, last.file) || first.hunk.Header != last.hunk.Header {
		return Selection{}, false
	}
	translated := next.BeginSelection(translatedFirst)
	return next.ExtendSelection(translated, translatedLast)
}
