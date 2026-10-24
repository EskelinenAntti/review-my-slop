package diffscreen

// replaceView translates interactive state before replacing the row projection.
// Source identity is shared by focus, selection endpoints, and the search
// origin.
func (v *View) replaceView(next *diffView) {
	old := v.view
	leftColumn := v.viewport.leftColumn
	rowsAbove := v.cursor.row - v.viewport.top
	cursor := identify(old, v.cursor)
	origin := identify(old, v.search.from)
	v.viewport = next.newViewport(v.viewport.width, v.viewport.height)
	v.viewport = next.scrollHorizontal(v.viewport, leftColumn)
	translated, ok := next.findCursor(cursor)
	if !ok {
		translated, _ = next.first()
	}
	v.cursor = translated
	v.selection = preserveSelection(old, v.selection, next)
	v.view = next
	if next.valid(v.cursor) {
		v.viewport.top = max(0, v.cursor.row-rowsAbove)
		v.viewport = next.keepVisible(v.viewport, v.cursor)
	}
	if v.search.active {
		v.search.from = v.cursor
		if translated, ok := next.findCursor(origin); ok {
			v.search.from = translated
		}
		v.previewSearch(v.search.query)
	}
}

func preserveSelection(
	old *diffView,
	selection *diffSelection,
	next *diffView,
) *diffSelection {
	if selection == nil {
		return nil
	}
	first := identify(old, selection.First)
	last := identify(old, selection.Last)
	if !first.valid || !last.valid {
		return nil
	}
	translatedFirst, firstOK := next.findCursor(first)
	translatedLast, lastOK := next.findCursor(last)
	if !firstOK || !lastOK || !sameFilePaths(
		first.oldPath,
		first.newPath,
		last.oldPath,
		last.newPath,
	) || first.hunkHeader != last.hunkHeader {
		return nil
	}
	translated, ok := next.selectionBetween(translatedFirst, translatedLast)
	if !ok {
		return nil
	}
	return &translated
}
