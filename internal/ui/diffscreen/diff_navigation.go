package diffscreen

import "iter"

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

// rowCursors visits valid panes in preference order. Unified rows retain the
// preferred pane identity but are visited only once.
func (v *diffView) rowCursors(y int, preferred diffPane) iter.Seq[diffCursor] {
	panes := []diffPane{preferred}
	if v.split {
		panes = append(panes, preferred.other())
	}
	return func(yield func(diffCursor) bool) {
		for _, pane := range panes {
			if cursor, ok := v.cursorAt(y, pane); ok && !yield(cursor) {
				return
			}
		}
	}
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

func (v *diffView) cursorNearMetadata(
	y int,
	pane diffPane,
	direction direction,
) (diffCursor, bool) {
	for candidateY := range neighborRows(y, len(v.rows)+1, direction) {
		if candidate, ok := v.cursorInFile(candidateY, y, pane); ok {
			return candidate, true
		}
	}
	return diffCursor{}, false
}

// neighborRows visits rows at increasing distances below limit, excluding the
// origin. Equidistant rows are visited in the preferred direction first.
// Callers apply their own file or viewport bounds.
func neighborRows(row, limit int, direction direction) iter.Seq[int] {
	return func(yield func(int) bool) {
		for distance := 1; distance < limit; distance++ {
			offset := int(direction) * distance
			if !yield(row+offset) || !yield(row-offset) {
				return
			}
		}
	}
}

func (v *diffView) cursorInFile(
	y, origin int,
	pane diffPane,
) (diffCursor, bool) {
	if y < 0 || y >= len(v.rows) || v.rows[y].file != v.rows[origin].file {
		return diffCursor{}, false
	}
	for candidate := range v.rowCursors(y, pane) {
		return candidate, true
	}
	return diffCursor{}, false
}

func (v *diffView) jumpFile(
	cursor diffCursor,
	direction direction,
) (diffCursor, bool) {
	if !v.valid(cursor) {
		return diffCursor{}, false
	}
	fileIndex := v.rows[cursor.row].file
	step := int(direction)
	for y := cursor.row + step; y >= 0 && y < len(v.rows); y += step {
		if v.rows[y].file == fileIndex {
			continue
		}
		if next, ok := v.cursorAt(y, cursor.pane); ok {
			return next, true
		}
	}
	return diffCursor{}, false
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
