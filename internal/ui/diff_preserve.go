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
func Preserve(old *view, state State, next *view) State {
	result := State{Viewport: next.NewViewport(state.Viewport.Width, state.Viewport.Height)}
	rowsAbove := 0
	if state.Cursor != nil {
		rowsAbove = state.Cursor.Coordinate.Y - state.Viewport.Top.Y
	}

	cursor := identify(old, state.Cursor)
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
		result.Viewport.Top.Y = max(0, result.Cursor.Coordinate.Y-rowsAbove)
		result.Viewport = next.KeepVisible(result.Viewport, *result.Cursor)
	}
	return result
}

func identify(v *view, cursor *Cursor) cursorIdentity {
	if cursor == nil {
		return cursorIdentity{}
	}
	file, fileOK := v.File(*cursor)
	hunk, hunkOK := v.Hunk(*cursor)
	line, lineOK := v.Line(*cursor)
	return cursorIdentity{file: file, hunk: hunk, line: line, cursor: *cursor, valid: fileOK && hunkOK && lineOK}
}

func preserveSelection(old *view, selection *Selection, next *view) (Selection, bool) {
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
