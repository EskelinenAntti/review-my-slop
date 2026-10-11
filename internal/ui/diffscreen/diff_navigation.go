package diffscreen

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

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
	if cursor, ok := v.scan(-1, right, forward); ok {
		return cursor, true
	}
	return v.scan(-1, left, forward)
}

func (v *diffView) last() (diffCursor, bool) {
	if cursor, ok := v.scan(len(v.rows), right, backward); ok {
		return cursor, true
	}
	return v.scan(len(v.rows), left, backward)
}

func (v *diffView) scan(start int, pane diffPane, direction direction) (
	diffCursor,
	bool,
) {
	for y := start + int(direction); y >= 0 && y < len(v.rows); y += int(
		direction,
	) {
		if cursor, ok := v.cursorAt(y, pane); ok {
			return cursor, true
		}
	}
	return diffCursor{}, false
}

func (v *diffView) move(cursor diffCursor, direction direction) (
	diffCursor,
	bool,
) {
	if !v.valid(cursor) {
		return diffCursor{}, false
	}
	return v.scan(cursor.row, cursor.pane, direction)
}

func (v *diffView) search(
	query string,
	cursor diffCursor,
	direction direction,
) (
	diffCursor,
	bool,
) {
	if query == "" || !v.valid(cursor) {
		return diffCursor{}, false
	}
	term := strings.ToLower(query)
	y := cursor.row
	for count := 0; count < len(v.rows)-1; count++ {
		y = wrappedRow(y+int(direction), len(v.rows))
		if candidate, ok := v.searchRow(term, y, cursor.pane, direction); ok {
			return candidate, true
		}
	}
	return diffCursor{}, false
}

func wrappedRow(row, count int) int {
	if row < 0 {
		return count - 1
	}
	if row >= count {
		return 0
	}
	return row
}

func (v *diffView) searchRow(
	term string,
	y int,
	pane diffPane,
	direction direction,
) (
	diffCursor,
	bool,
) {
	if candidate, ok := v.searchCodeRow(term, y, pane); ok {
		return candidate, true
	}
	current := v.rows[y]
	if current.kind == lineRow {
		return diffCursor{}, false
	}
	if strings.Contains(strings.ToLower(ansi.Strip(current.text)), term) {
		return v.cursorNearRow(y, pane, direction)
	}
	return diffCursor{}, false
}

func (v *diffView) searchCodeRow(term string, y int, preferred diffPane) (
	diffCursor,
	bool,
) {
	panes := []diffPane{preferred}
	if v.split {
		panes = append(panes, preferred.other())
	}
	for _, pane := range panes {
		candidate, ok := v.cursorAt(y, pane)
		if !ok {
			continue
		}
		line, _ := v.line(candidate)
		if strings.Contains(strings.ToLower(line.Text), term) {
			return candidate, true
		}
	}
	return diffCursor{}, false
}

func (v *diffView) cursorNearRow(y int, pane diffPane, direction direction) (
	diffCursor,
	bool,
) {
	for distance := 1; distance <= len(v.rows); distance++ {
		candidate, ok := v.nearRowAtDistance(y, pane, direction, distance)
		if ok {
			return candidate, true
		}
	}
	return diffCursor{}, false
}

func (v *diffView) rowInFile(row, file int) bool {
	return row >= 0 && row < len(v.rows) && v.rows[row].file == file
}

func (v *diffView) cursorInEitherPane(y int, pane diffPane) (diffCursor, bool) {
	if candidate, ok := v.cursorAt(y, pane); ok {
		return candidate, true
	}
	if v.split {
		return v.cursorAt(y, pane.other())
	}
	return diffCursor{}, false
}

func (v *diffView) jumpFile(cursor diffCursor, direction direction) (
	diffCursor,
	bool,
) {
	if !v.valid(cursor) {
		return diffCursor{}, false
	}
	file, _ := v.file(cursor)
	y := cursor.row
	for {
		next, ok := v.scan(y, cursor.pane, direction)
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

func (v *diffView) switchPane(cursor diffCursor, pane diffPane) (
	diffCursor,
	bool,
) {
	if !v.split || !v.valid(cursor) {
		return diffCursor{}, false
	}
	if candidate, ok := v.cursorAt(cursor.row, pane); ok {
		return candidate, true
	}
	if candidate, ok := v.previousCursorInFile(cursor.row, pane); ok {
		return candidate, true
	}
	return v.scan(cursor.row, pane, forward)
}

func (v *diffView) previousCursorInFile(row int, pane diffPane) (
	diffCursor,
	bool,
) {
	for y := row - 1; y >= 0; y-- {
		if v.rows[y].file != v.rows[row].file {
			break
		}
		if candidate, ok := v.cursorAt(y, pane); ok {
			return candidate, true
		}
	}
	return diffCursor{}, false
}

func (v *diffView) nearRowAtDistance(
	y int,
	pane diffPane,
	direction direction,
	distance int,
) (
	diffCursor,
	bool,
) {
	for _, candidateY := range []int{
		y + int(direction)*distance,
		y - int(direction)*distance,
	} {
		if !v.rowInFile(candidateY, v.rows[y].file) {
			continue
		}
		if candidate, ok := v.cursorInEitherPane(candidateY, pane); ok {
			return candidate, true
		}
	}
	return diffCursor{}, false
}
