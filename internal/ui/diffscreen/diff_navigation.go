package diffscreen

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

func (v *diffView) hasSourceLine(cursor diffCursor) bool {
	return cursor.row >= 0 && cursor.row < len(v.rows) &&
		v.sourceLineIndexInHunk(v.rows[cursor.row], cursor.pane) >= 0
}

func (v *diffView) sourceLineIndexInHunk(current entry, pane diffPane) int {
	if current.kind != lineRow {
		return -1
	}
	if !v.split || pane == right {
		return current.rightLine
	}
	return current.leftLine
}

func (v *diffView) sourceCursorAtRow(y int, pane diffPane) (diffCursor, bool) {
	cursor := diffCursor{row: y, pane: pane}
	return cursor, v.hasSourceLine(cursor)
}

func (v *diffView) firstLineCursor() (diffCursor, bool) {
	if cursor, ok := v.scanSourceLineCursor(-1, right, forward); ok {
		return cursor, true
	}
	return v.scanSourceLineCursor(-1, left, forward)
}

func (v *diffView) lastLineCursor() (diffCursor, bool) {
	if cursor, ok := v.scanSourceLineCursor(len(v.rows), right, backward); ok {
		return cursor, true
	}
	return v.scanSourceLineCursor(len(v.rows), left, backward)
}

func (v *diffView) scanSourceLineCursor(
	start int,
	pane diffPane,
	direction direction,
) (diffCursor, bool) {
	step := int(direction)
	for y := start + step; y >= 0 && y < len(v.rows); y += step {
		if cursor, ok := v.sourceCursorAtRow(y, pane); ok {
			return cursor, true
		}
	}
	return diffCursor{}, false
}

func (v *diffView) adjacentLineCursor(
	cursor diffCursor,
	direction direction,
) (diffCursor, bool) {
	if !v.hasSourceLine(cursor) {
		return diffCursor{}, false
	}
	return v.scanSourceLineCursor(cursor.row, cursor.pane, direction)
}

func (v *diffView) search(
	query string,
	cursor diffCursor,
	direction direction,
) (diffCursor, bool) {
	if query == "" || !v.hasSourceLine(cursor) {
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
	panes := []diffPane{pane}
	if v.split {
		panes = append(panes, pane.oppositePane())
	}
	for _, candidatePane := range panes {
		candidate, ok := v.sourceCursorAtRow(y, candidatePane)
		if !ok {
			continue
		}
		line, _ := v.line(candidate)
		if strings.Contains(strings.ToLower(line.Text), query) {
			return candidate, true
		}
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
		return v.nearestLineCursorInFile(y, pane, direction)
	}
	return diffCursor{}, false
}

func (v *diffView) nearestLineCursorInFile(
	y int,
	pane diffPane,
	direction direction,
) (diffCursor, bool) {
	for distance := 1; distance <= len(v.rows); distance++ {
		for _, candidateY := range []int{
			y + int(direction)*distance,
			y - int(direction)*distance,
		} {
			if candidate, ok := v.cursorInFile(candidateY, y, pane); ok {
				return candidate, true
			}
		}
	}
	return diffCursor{}, false
}

func (v *diffView) cursorInFile(
	y, originRow int,
	pane diffPane,
) (diffCursor, bool) {
	if y < 0 || y >= len(v.rows) || v.rows[y].file != v.rows[originRow].file {
		return diffCursor{}, false
	}
	if candidate, ok := v.sourceCursorAtRow(y, pane); ok {
		return candidate, true
	}
	if v.split {
		return v.sourceCursorAtRow(y, pane.oppositePane())
	}
	return diffCursor{}, false
}

func (v *diffView) jumpFile(
	cursor diffCursor,
	direction direction,
) (diffCursor, bool) {
	if !v.hasSourceLine(cursor) {
		return diffCursor{}, false
	}
	file, _ := v.file(cursor)
	y := cursor.row
	for {
		next, ok := v.scanSourceLineCursor(y, cursor.pane, direction)
		if !ok {
			return diffCursor{}, false
		}
		nextFile, _ := v.file(next)
		if nextFile.OldPath != file.OldPath ||
			nextFile.NewPath != file.NewPath {
			return next, true
		}
		y = next.row
	}
}

func (v *diffView) switchPane(
	cursor diffCursor,
	pane diffPane,
) (diffCursor, bool) {
	if !v.split || !v.hasSourceLine(cursor) {
		return diffCursor{}, false
	}
	if candidate, ok := v.sourceCursorAtRow(cursor.row, pane); ok {
		return candidate, true
	}
	if candidate, ok := v.previousLineCursorInFile(cursor, pane); ok {
		return candidate, true
	}
	return v.scanSourceLineCursor(cursor.row, pane, forward)
}

func (v *diffView) previousLineCursorInFile(
	cursor diffCursor,
	pane diffPane,
) (diffCursor, bool) {
	for y := cursor.row - 1; y >= 0; y-- {
		if v.rows[y].file != v.rows[cursor.row].file {
			break
		}
		if candidate, ok := v.sourceCursorAtRow(y, pane); ok {
			return candidate, true
		}
	}
	return diffCursor{}, false
}
