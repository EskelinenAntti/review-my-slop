package ui

import "github.com/eskelinenantti/review-my-slop/internal/patch"

type cursorIdentity struct {
	file   patch.File
	hunk   patch.Hunk
	line   patch.Line
	cursor Cursor
	valid  bool
}

// Preserve returns fresh State for next by retaining the meaningful position
// from state where next contains it.
func Preserve(old View, state State, next View) State {
	viewport, oldCursor := state.Viewport, state.Cursor
	result := State{Viewport: next.Resize(Viewport{}, viewport.Width, viewport.Height)}
	rowsAbove := 0
	if oldCursor != nil {
		rowsAbove = oldCursor.Coordinate - viewport.Top
	}

	cursor := identify(old, oldCursor)
	if cursor.valid {
		if translated, ok := next.FindCursor(cursor.file, cursor.hunk, cursor.line, cursor.cursor.Coordinate, cursor.cursor.Pane); ok {
			result.Cursor = &translated
		}
	}
	if result.Cursor == nil {
		if first, ok := next.First(); ok {
			result.Cursor = &first
		}
	}

	if selection, ok := preserveSelection(old, state.Selection, next); ok {
		result.Selection = &selection
	}
	if result.Cursor != nil {
		result.Viewport.Top = max(0, result.Cursor.Coordinate-rowsAbove)
		result.Viewport = next.KeepVisible(result.Viewport, *result.Cursor)
	}
	return result
}

func identify(v View, cursor *Cursor) cursorIdentity {
	if cursor == nil || !v.valid(*cursor) {
		return cursorIdentity{}
	}
	current := v.rows[cursor.Coordinate]
	file := v.patch.Files[current.file]
	hunk := file.Hunks[current.hunk]
	line := hunk.Lines[v.lineIndex(current, cursor.Pane)]
	return cursorIdentity{file, hunk, line, *cursor, true}
}

func preserveSelection(old View, selection *Selection, next View) (Selection, bool) {
	if selection == nil {
		return Selection{}, false
	}
	first := identify(old, &selection.First)
	last := identify(old, &selection.Last)
	if !first.valid || !last.valid {
		return Selection{}, false
	}
	translatedFirst, firstOK := next.FindCursor(first.file, first.hunk, first.line, first.cursor.Coordinate, first.cursor.Pane)
	translatedLast, lastOK := next.FindCursor(last.file, last.hunk, last.line, last.cursor.Coordinate, last.cursor.Pane)
	if !firstOK || !lastOK || !sameFile(first.file, last.file) || first.hunk.Header != last.hunk.Header {
		return Selection{}, false
	}
	translated := next.BeginSelection(translatedFirst)
	return next.ExtendSelection(translated, translatedLast)
}
