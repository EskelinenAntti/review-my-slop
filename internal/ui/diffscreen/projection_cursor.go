package diffscreen

import "github.com/eskelinenantti/review-my-slop/internal/patch"

func (v *patchProjection) validCursor(cursor patchCursor) bool {
	return cursor.row >= 0 && cursor.row < len(v.rows) &&
		v.sourceLineIndex(v.rows[cursor.row], cursor.pane) >= 0
}

func (v *patchProjection) fileAtCursor(cursor patchCursor) (patch.File, bool) {
	if !v.validCursor(cursor) {
		return patch.File{}, false
	}
	return v.patch.Files[v.rows[cursor.row].file], true
}

func (v *patchProjection) lineAtCursor(cursor patchCursor) (patch.Line, bool) {
	if !v.validCursor(cursor) {
		return patch.Line{}, false
	}
	current := v.rows[cursor.row]
	lineIndex := v.sourceLineIndex(current, cursor.pane)
	hunk := v.patch.Files[current.file].Hunks[current.hunk]
	return hunk.Lines[lineIndex], true
}

func (v *patchProjection) sourceLineIndex(
	current displayRow,
	pane patchPane,
) int {
	if current.kind != lineRow {
		return -1
	}
	if !v.split || pane == right {
		return current.rightLine
	}
	return current.leftLine
}

func (v *patchProjection) cursorOnRow(
	y int,
	pane patchPane,
) (patchCursor, bool) {
	cursor := patchCursor{row: y, pane: pane}
	return cursor, v.validCursor(cursor)
}

func (v *patchProjection) firstCursor() (patchCursor, bool) {
	if cursor, ok := v.scanCursors(-1, right, forward); ok {
		return cursor, true
	}
	return v.scanCursors(-1, left, forward)
}

func (v *patchProjection) lastCursor() (patchCursor, bool) {
	if cursor, ok := v.scanCursors(len(v.rows), right, backward); ok {
		return cursor, true
	}
	return v.scanCursors(len(v.rows), left, backward)
}

func (v *patchProjection) scanCursors(
	start int,
	pane patchPane,
	direction direction,
) (patchCursor, bool) {
	step := int(direction)
	for y := start + step; y >= 0 && y < len(v.rows); y += step {
		if cursor, ok := v.cursorOnRow(y, pane); ok {
			return cursor, true
		}
	}
	return patchCursor{}, false
}

func (v *patchProjection) moveCursor(
	cursor patchCursor,
	direction direction,
) (patchCursor, bool) {
	if !v.validCursor(cursor) {
		return patchCursor{}, false
	}
	return v.scanCursors(cursor.row, cursor.pane, direction)
}

func (v *patchProjection) nearestCursorNearRow(
	y int,
	pane patchPane,
	direction direction,
) (patchCursor, bool) {
	for distance := 1; distance <= len(v.rows); distance++ {
		for _, candidateY := range []int{
			y + int(direction)*distance,
			y - int(direction)*distance,
		} {
			if candidate, ok := v.cursorOnRowInFile(candidateY, y, pane); ok {
				return candidate, true
			}
		}
	}
	return patchCursor{}, false
}

func (v *patchProjection) cursorOnRowInFile(
	y, origin int,
	pane patchPane,
) (patchCursor, bool) {
	if y < 0 || y >= len(v.rows) || v.rows[y].file != v.rows[origin].file {
		return patchCursor{}, false
	}
	if candidate, ok := v.cursorOnRow(y, pane); ok {
		return candidate, true
	}
	if v.split {
		return v.cursorOnRow(y, pane.other())
	}
	return patchCursor{}, false
}

func (v *patchProjection) jumpToAdjacentFile(
	cursor patchCursor,
	direction direction,
) (patchCursor, bool) {
	if !v.validCursor(cursor) {
		return patchCursor{}, false
	}
	file, _ := v.fileAtCursor(cursor)
	y := cursor.row
	for {
		next, ok := v.scanCursors(y, cursor.pane, direction)
		if !ok {
			return patchCursor{}, false
		}
		nextFile, _ := v.fileAtCursor(next)
		if nextFile.OldPath != file.OldPath ||
			nextFile.NewPath != file.NewPath {
			return next, true
		}
		y = next.row
	}
}

func (v *patchProjection) switchCursorPane(
	cursor patchCursor,
	pane patchPane,
) (patchCursor, bool) {
	if !v.split || !v.validCursor(cursor) {
		return patchCursor{}, false
	}
	if candidate, ok := v.cursorOnRow(cursor.row, pane); ok {
		return candidate, true
	}
	if candidate, ok := v.previousCursorInFile(cursor, pane); ok {
		return candidate, true
	}
	return v.scanCursors(cursor.row, pane, forward)
}

func (v *patchProjection) previousCursorInFile(
	cursor patchCursor,
	pane patchPane,
) (patchCursor, bool) {
	for y := cursor.row - 1; y >= 0; y-- {
		if v.rows[y].file != v.rows[cursor.row].file {
			break
		}
		if candidate, ok := v.cursorOnRow(y, pane); ok {
			return candidate, true
		}
	}
	return patchCursor{}, false
}
