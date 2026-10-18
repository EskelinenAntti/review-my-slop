package diffscreen

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

func (v *diffView) validCursor(cursor diffCursor) bool {
	return cursor.row >= 0 && cursor.row < len(v.rows) &&
		v.hunkLineIndex(v.rows[cursor.row], cursor.pane) >= 0
}

func (v *diffView) hunkLineIndex(current entry, pane diffPane) int {
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
	return cursor, v.validCursor(cursor)
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
	if !v.validCursor(cursor) {
		return diffCursor{}, false
	}
	return v.scan(cursor.row, cursor.pane, direction)
}

func (v *diffView) search(
	query string,
	cursor diffCursor,
	direction direction,
) (diffCursor, bool) {
	if query == "" || !v.validCursor(cursor) {
		return diffCursor{}, false
	}
	query = strings.ToLower(query)
	y := cursor.row
	for count := 0; count < len(v.rows)-1; count++ {
		y = (y + int(direction) + len(v.rows)) % len(v.rows)
		if candidate, ok := v.searchRow(query, y, cursor.pane, direction); ok {
			return candidate, true
		}
	}
	return diffCursor{}, false
}

func (v *diffView) searchRow(
	query string,
	y int,
	pane diffPane,
	direction direction,
) (diffCursor, bool) {
	if v.rows[y].kind != lineRow {
		return v.searchMetadata(query, y, pane, direction)
	}
	if candidate, ok := v.searchPane(query, y, pane); ok {
		return candidate, true
	}
	if v.split {
		return v.searchPane(query, y, pane.other())
	}
	return diffCursor{}, false
}

func (v *diffView) searchPane(
	query string,
	y int,
	pane diffPane,
) (diffCursor, bool) {
	candidate, ok := v.cursorAt(y, pane)
	if !ok {
		return diffCursor{}, false
	}
	line, _ := v.line(candidate)
	if strings.Contains(strings.ToLower(line.Text), query) {
		return candidate, true
	}
	return diffCursor{}, false
}

func (v *diffView) searchMetadata(
	query string,
	y int,
	pane diffPane,
	direction direction,
) (diffCursor, bool) {
	if strings.Contains(strings.ToLower(ansi.Strip(v.rows[y].text)), query) {
		return v.cursorNearMetadata(y, pane, direction)
	}
	return diffCursor{}, false
}

func (v *diffView) cursorNearMetadata(
	y int,
	pane diffPane,
	direction direction,
) (diffCursor, bool) {
	for distance := 1; distance <= len(v.rows); distance++ {
		for _, candidateY := range neighborRows(y, distance, direction) {
			if candidate, ok := v.cursorInFile(candidateY, y, pane); ok {
				return candidate, true
			}
		}
	}
	return diffCursor{}, false
}

func neighborRows(row, distance int, direction direction) [2]int {
	offset := int(direction) * distance
	return [2]int{row + offset, row - offset}
}

func (v *diffView) cursorInFile(
	y, origin int,
	pane diffPane,
) (diffCursor, bool) {
	if y < 0 || y >= len(v.rows) || v.rows[y].file != v.rows[origin].file {
		return diffCursor{}, false
	}
	if candidate, ok := v.cursorAt(y, pane); ok {
		return candidate, true
	}
	if v.split {
		return v.cursorAt(y, pane.other())
	}
	return diffCursor{}, false
}

func (v *diffView) jumpFile(
	cursor diffCursor,
	direction direction,
) (diffCursor, bool) {
	if !v.validCursor(cursor) {
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
	if !v.split || !v.validCursor(cursor) {
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
