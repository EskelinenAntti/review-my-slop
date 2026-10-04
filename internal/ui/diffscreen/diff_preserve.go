package diffscreen

import "github.com/eskelinenantti/review-my-slop/internal/patch"

type cursorIdentity struct {
	file   patch.File
	hunk   patch.Hunk
	line   patch.Line
	cursor diffCursor
	valid  bool
}

// preserve returns fresh viewState for next by retaining the meaningful position
// from state where next contains it.
func preserve(old *diffView, state viewState, next *diffView) viewState {
	result := viewState{viewport: next.newViewport(state.viewport.Width, state.viewport.Height)}
	result.viewport = next.scrollHorizontal(result.viewport, state.viewport.LeftColumn)
	rowsAbove := 0
	if state.cursor != nil {
		rowsAbove = state.cursor.coordinate.Y - state.viewport.top.Y
	}

	cursor := identify(old, state.cursor)
	if cursor.valid {
		if translated, ok := next.findCursor(cursor.file, cursor.hunk, cursor.line, cursor.cursor.coordinate, cursor.cursor.pane); ok {
			result.cursor = &translated
		}
	}
	if result.cursor == nil {
		if first, ok := next.first(); ok {
			result.cursor = &first
		}
	}

	if selection, ok := preserveSelection(old, state.selection, next); ok {
		result.selection = &selection
	}
	if result.cursor != nil {
		result.viewport.top.Y = max(0, result.cursor.coordinate.Y-rowsAbove)
		result.viewport = next.keepVisible(result.viewport, *result.cursor)
	}
	return result
}

func identify(v *diffView, cursor *diffCursor) cursorIdentity {
	if cursor == nil {
		return cursorIdentity{}
	}
	file, fileOK := v.file(*cursor)
	hunk, hunkOK := v.hunk(*cursor)
	line, lineOK := v.line(*cursor)
	return cursorIdentity{file: file, hunk: hunk, line: line, cursor: *cursor, valid: fileOK && hunkOK && lineOK}
}

func preserveSelection(old *diffView, selection *diffSelection, next *diffView) (diffSelection, bool) {
	if selection == nil {
		return diffSelection{}, false
	}
	first := identify(old, &selection.First)
	last := identify(old, &selection.Last)
	if !first.valid || !last.valid {
		return diffSelection{}, false
	}
	translatedFirst, firstOK := next.findCursor(first.file, first.hunk, first.line, first.cursor.coordinate, first.cursor.pane)
	translatedLast, lastOK := next.findCursor(last.file, last.hunk, last.line, last.cursor.coordinate, last.cursor.pane)
	if !firstOK || !lastOK || !sameFile(first.file, last.file) || first.hunk.Header != last.hunk.Header {
		return diffSelection{}, false
	}
	translated := next.beginSelection(translatedFirst)
	return next.extendSelection(translated, translatedLast)
}
