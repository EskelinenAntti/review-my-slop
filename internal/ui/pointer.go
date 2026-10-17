package ui

func pointerRow(y, width, height int) (int, bool) {
	row := y - 1
	return row, width > 0 && height > 0 && row >= 0 && row < max(
		1,
		height-3,
	) && y < height-2
}

func pointerPane(view *diffView,
	viewport diffViewport, x int) (diffPane, bool) {
	if !view.split {
		return right, true
	}
	leftWidth := max(20, (viewport.Width-3)/2)
	switch {
	case x < leftWidth:
		return left, true
	case x < leftWidth+3:
		return right, false
	default:
		return right, true
	}
}

func cursorLine(view *diffView,
	cursor diffCursor) int {
	if cursor.row < 0 || cursor.row >= len(view.rows) {
		return -1
	}
	row := view.rows[cursor.row]
	if row.kind != lineRow {
		return -1
	}
	if !view.split || cursor.pane == right {
		return row.rightLine
	}
	return row.leftLine
}

func pointerCursor(
	view *diffView,
	viewport diffViewport,
	x, y, width, height int,
) (diffCursor, bool) {
	if x < 0 || x >= width {
		return diffCursor{}, false
	}
	row, ok := pointerRow(y, width, height)
	if !ok {
		return diffCursor{}, false
	}
	row, ok = visibleContentRow(view, viewport, row)
	if !ok {
		return diffCursor{}, false
	}
	pane, ok := pointerPane(view, viewport, x)
	if !ok {
		return diffCursor{}, false
	}
	cursor := diffCursor{row: viewport.top + row, pane: pane}
	return cursor, cursorLine(view, cursor) >= 0
}

func visibleContentRow(
	view *diffView,
	viewport diffViewport,
	row int,
) (int, bool) {
	if viewportHasFileHeader(view, viewport) {
		row--
	}
	if row < 0 || row >= max(1, viewportContentHeight(
		view,
		viewport,
	)) {
		return 0, false
	}
	return row, true
}

func viewportHasFileHeader(view *diffView,
	viewport diffViewport) bool {
	return viewport.Height > 1 && viewport.top >= 0 &&
		viewport.top < len(view.rows) &&
		view.rows[viewport.top].kind != fileRow
}

func viewportContentHeight(view *diffView,
	viewport diffViewport) int {
	height := viewport.Height
	if viewportHasFileHeader(view, viewport) {
		height--
	}
	return height
}

func selectionForDrag(view *diffView, first,
	last diffCursor) (diffSelection, bool) {
	if cursorLine(
		view,
		first,
	) < 0 || cursorLine(
		view,
		last,
	) < 0 {
		return diffSelection{}, false
	}
	firstRow, lastRow := view.rows[first.row], view.rows[last.row]
	if firstRow.file != lastRow.file || firstRow.hunk != lastRow.hunk {
		return diffSelection{}, false
	}
	return diffSelection{First: first, Last: last}, true
}
