package diffscreen

import "iter"

func (v *diffView) cursorAt(y int, pane diffPane) (diffCursor, bool) {
	cursor := diffCursor{row: y, pane: pane}
	return cursor, v.valid(cursor)
}

// cursorsAt yields selectable cells in preferred-pane order. Unified rows
// retain the preferred pane without visiting the same source line twice.
func (v *diffView) cursorsAt(y int, preferred diffPane) iter.Seq[diffCursor] {
	return func(yield func(diffCursor) bool) {
		if cursor, ok := v.cursorAt(y, preferred); ok {
			if !yield(cursor) {
				return
			}
		}
		if v.split {
			if cursor, ok := v.cursorAt(y, preferred.other()); ok {
				yield(cursor)
			}
		}
	}
}

func (v *diffView) first() (diffCursor, bool) {
	return v.edge(Forward)
}

func (v *diffView) last() (diffCursor, bool) {
	return v.edge(Backward)
}

func (v *diffView) edge(direction Direction) (diffCursor, bool) {
	start := -1
	if direction == Backward {
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
	direction Direction,
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
	direction Direction,
) (diffCursor, bool) {
	if !v.valid(cursor) {
		return diffCursor{}, false
	}
	return v.scan(cursor.row, cursor.pane, direction)
}

func neighborRows(row, distance int, direction Direction) [2]int {
	offset := int(direction) * distance
	return [2]int{row + offset, row - offset}
}

func (v *diffView) jumpFile(
	cursor diffCursor,
	direction Direction,
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
	return v.scan(cursor.row, pane, Forward)
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
