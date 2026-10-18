package diffscreen

func (v *diffView) valid(cursor diffCursor) bool {
	return cursor.row >= 0 && cursor.row < len(v.rows) &&
		v.lineIndex(v.rows[cursor.row], cursor.pane) >= 0
}

func (v *diffView) lineIndex(current entry, pane diffPane) int {
	if current.kind != lineRow {
		return -1
	}
	if !v.split || pane == right {
		return current.rightLine
	}
	return current.leftLine
}

func (v *diffView) cursorAt(y int, pane diffPane) (diffCursor, bool) {
	cursor := diffCursor{row: y, pane: pane}
	return cursor, v.valid(cursor)
}

func (v *diffView) first() (diffCursor, bool) {
	return v.edge(forward)
}

func (v *diffView) last() (diffCursor, bool) {
	return v.edge(backward)
}

func (v *diffView) edge(direction direction) (diffCursor, bool) {
	start := -1
	if direction == backward {
		start = len(v.rows)
	}
	if cursor, ok := v.scan(start, right, direction); ok {
		return cursor, true
	}
	return v.scan(start, left, direction)
}

func (v *diffView) scan(
	start int,
	pane diffPane,
	direction direction,
) (diffCursor, bool) {
	step := int(direction)
	for y := start + step; y >= 0 && y < len(v.rows); y += step {
		if cursor, ok := v.cursorAt(y, pane); ok {
			return cursor, true
		}
	}
	return diffCursor{}, false
}

func (v *diffView) move(
	cursor diffCursor,
	direction direction,
) (diffCursor, bool) {
	if !v.valid(cursor) {
		return diffCursor{}, false
	}
	return v.scan(cursor.row, cursor.pane, direction)
}

func (v *diffView) jumpFile(
	cursor diffCursor,
	direction direction,
) (diffCursor, bool) {
	if !v.valid(cursor) {
		return diffCursor{}, false
	}
	fileIndex := v.rows[cursor.row].file
	y := cursor.row
	for {
		next, ok := v.scan(y, cursor.pane, direction)
		if !ok {
			return diffCursor{}, false
		}
		if v.rows[next.row].file != fileIndex {
			return next, true
		}
		y = next.row
	}
}

func (v *diffView) switchPane(
	cursor diffCursor,
	pane diffPane,
) (diffCursor, bool) {
	if !v.split || !v.valid(cursor) {
		return diffCursor{}, false
	}
	if candidate, ok := v.cursorAt(cursor.row, pane); ok {
		return candidate, true
	}
	if candidate, ok := v.previousInFile(cursor, pane); ok {
		return candidate, true
	}
	return v.scan(cursor.row, pane, forward)
}

func (v *diffView) previousInFile(
	cursor diffCursor,
	pane diffPane,
) (diffCursor, bool) {
	for y := cursor.row - 1; y >= 0; y-- {
		if v.rows[y].file != v.rows[cursor.row].file {
			break
		}
		if candidate, ok := v.cursorAt(y, pane); ok {
			return candidate, true
		}
	}
	return diffCursor{}, false
}
