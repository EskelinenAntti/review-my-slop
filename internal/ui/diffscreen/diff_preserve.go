package diffscreen

// replaceProjection translates interactive state before replacing the row projection.
// Source identity is shared by focus, selection endpoints, and the search origin.
func (v *View) replaceProjection(next *projection) {
	old := v.projection
	leftColumn := v.viewport.LeftColumn
	rowsAbove := v.cursor.row - v.viewport.top
	cursor := old.sourceAt(v.cursor)
	origin := old.sourceAt(v.search.from)
	v.viewport = next.newViewport(v.viewport.Width, v.viewport.Height)
	v.viewport = next.scrollHorizontal(v.viewport, leftColumn)
	translated, ok := next.restoreCursor(cursor)
	if !ok {
		translated, _ = next.firstCursor()
	}
	v.cursor = translated
	v.selection = preserveSelection(old, v.selection, next)
	v.projection = next
	if next.hasSourceLine(v.cursor) {
		v.viewport.top = max(0, v.cursor.row-rowsAbove)
		v.viewport = next.keepVisible(v.viewport, v.cursor)
	}
	if v.search.active {
		v.search.from = v.cursor
		if translated, ok := next.restoreCursor(origin); ok {
			v.search.from = translated
		}
		v.previewSearch(v.search.query)
	}
}

func preserveSelection(old *projection, selection *diffSelection, next *projection) *diffSelection {
	if selection == nil {
		return nil
	}
	first := old.sourceAt(selection.First)
	last := old.sourceAt(selection.Last)
	if !first.valid || !last.valid {
		return nil
	}
	translatedFirst, firstOK := next.restoreCursor(first)
	translatedLast, lastOK := next.restoreCursor(last)
	if !firstOK || !lastOK || !sameFile(first.file, last.file) || first.hunk.Header != last.hunk.Header {
		return nil
	}
	translated := diffSelection{First: translatedFirst, Last: translatedFirst}
	translated, ok := next.extendSelection(translated, translatedLast)
	if !ok {
		return nil
	}
	return &translated
}
