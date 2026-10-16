package diffscreen

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

func (v *diffView) valid(cursor diffCursor) bool {
	return cursor.row >= 0 && cursor.row < len(v.rows) &&
		v.sourceLineIndex(v.rows[cursor.row], cursor.pane) >= 0
}

func (v *diffView) sourceLineIndex(
	displayRow entry,
	pane diffPane,
) (sourceLineIndex int) {
	if displayRow.kind != lineRow {
		return -1
	}
	if !v.split || pane == right {
		return displayRow.rightLine
	}
	return displayRow.leftLine
}

func (v *diffView) cursorAt(row int, pane diffPane) (cursor diffCursor, found bool) {
	cursor = diffCursor{row: row, pane: pane}
	return cursor, v.valid(cursor)
}

func (v *diffView) first() (cursor diffCursor, found bool) {
	if cursor, ok := v.findCursorFromRow(-1, right, forward); ok {
		return cursor, true
	}
	return v.findCursorFromRow(-1, left, forward)
}

func (v *diffView) last() (cursor diffCursor, found bool) {
	if cursor, ok := v.findCursorFromRow(len(v.rows), right, backward); ok {
		return cursor, true
	}
	return v.findCursorFromRow(len(v.rows), left, backward)
}

func (v *diffView) findCursorFromRow(
	startRow int,
	pane diffPane,
	direction direction,
) (cursor diffCursor, found bool) {
	step := int(direction)
	for row := startRow + step; row >= 0 && row < len(v.rows); row += step {
		if cursor, ok := v.cursorAt(row, pane); ok {
			return cursor, true
		}
	}
	return diffCursor{}, false
}

func (v *diffView) move(
	cursor diffCursor,
	direction direction,
) (nextCursor diffCursor, found bool) {
	if !v.valid(cursor) {
		return diffCursor{}, false
	}
	return v.findCursorFromRow(cursor.row, cursor.pane, direction)
}

func (v *diffView) search(
	query string,
	cursor diffCursor,
	direction direction,
) (match diffCursor, found bool) {
	if query == "" || !v.valid(cursor) {
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
	row int,
	pane diffPane,
	direction direction,
) (match diffCursor, found bool) {
	if v.rows[row].kind != lineRow {
		return v.searchMetadata(query, row, pane, direction)
	}
	panes := []diffPane{pane}
	if v.split {
		panes = append(panes, pane.otherPane())
	}
	for _, candidatePane := range panes {
		candidate, ok := v.cursorAt(row, candidatePane)
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
	row int,
	pane diffPane,
	direction direction,
) (match diffCursor, found bool) {
	if strings.Contains(strings.ToLower(ansi.Strip(v.rows[row].text)), query) {
		return v.cursorNearRow(row, pane, direction)
	}
	return diffCursor{}, false
}

func (v *diffView) cursorNearRow(
	row int,
	pane diffPane,
	direction direction,
) (cursor diffCursor, found bool) {
	for distance := 1; distance <= len(v.rows); distance++ {
		for _, candidateY := range []int{
			row + int(direction)*distance,
			row - int(direction)*distance,
		} {
			if candidate, ok := v.cursorAtRowInSameFile(candidateY, row, pane); ok {
				return candidate, true
			}
		}
	}
	return diffCursor{}, false
}

func (v *diffView) cursorAtRowInSameFile(
	candidateRow, referenceRow int,
	pane diffPane,
) (cursor diffCursor, found bool) {
	if candidateRow < 0 || candidateRow >= len(v.rows) ||
		referenceRow < 0 || referenceRow >= len(v.rows) ||
		v.rows[candidateRow].file != v.rows[referenceRow].file {
		return diffCursor{}, false
	}
	if candidate, ok := v.cursorAt(candidateRow, pane); ok {
		return candidate, true
	}
	if v.split {
		return v.cursorAt(candidateRow, pane.otherPane())
	}
	return diffCursor{}, false
}

func (v *diffView) jumpFile(
	cursor diffCursor,
	direction direction,
) (nextCursor diffCursor, found bool) {
	if !v.valid(cursor) {
		return diffCursor{}, false
	}
	file, _ := v.file(cursor)
	y := cursor.row
	for {
		next, ok := v.findCursorFromRow(y, cursor.pane, direction)
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
) (nextCursor diffCursor, found bool) {
	if !v.split || !v.valid(cursor) {
		return diffCursor{}, false
	}
	if candidate, ok := v.cursorAt(cursor.row, pane); ok {
		return candidate, true
	}
	if candidate, ok := v.previousInFile(cursor, pane); ok {
		return candidate, true
	}
	return v.findCursorFromRow(cursor.row, pane, forward)
}

func (v *diffView) previousInFile(
	cursor diffCursor,
	pane diffPane,
) (previousCursor diffCursor, found bool) {
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
