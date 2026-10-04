package diffscreen

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

func (v *diffView) valid(cursor diffCursor) bool {
	return cursor.row >= 0 && cursor.row < len(v.rows) && v.lineIndex(v.rows[cursor.row], cursor.pane) >= 0
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

func (v *diffView) scan(start int, pane diffPane, direction direction) (diffCursor, bool) {
	for y := start + int(direction); y >= 0 && y < len(v.rows); y += int(direction) {
		if cursor, ok := v.cursorAt(y, pane); ok {
			return cursor, true
		}
	}
	return diffCursor{}, false
}

func (v *diffView) move(cursor diffCursor, direction direction) (diffCursor, bool) {
	if !v.valid(cursor) {
		return diffCursor{}, false
	}
	return v.scan(cursor.row, cursor.pane, direction)
}

func (v *diffView) search(query string, cursor diffCursor, direction direction) (diffCursor, bool) {
	if query == "" || !v.valid(cursor) {
		return diffCursor{}, false
	}
	query = strings.ToLower(query)
	y := cursor.row
	for count := 0; count < len(v.rows)-1; count++ {
		y += int(direction)
		if y < 0 {
			y = len(v.rows) - 1
		}
		if y >= len(v.rows) {
			y = 0
		}
		current := v.rows[y]
		for _, pane := range []diffPane{cursor.pane, cursor.pane.other()} {
			candidate, ok := v.cursorAt(y, pane)
			if !ok || !v.split && pane != cursor.pane {
				continue
			}
			line, _ := v.line(candidate)
			if strings.Contains(strings.ToLower(line.Text), query) {
				return candidate, true
			}
		}
		if current.kind != lineRow && strings.Contains(strings.ToLower(ansi.Strip(current.text)), query) {
			if candidate, ok := v.cursorNearRow(y, cursor.pane, direction); ok {
				return candidate, true
			}
		}
	}
	return diffCursor{}, false
}

func (v *diffView) cursorNearRow(y int, pane diffPane, direction direction) (diffCursor, bool) {
	for distance := 1; distance <= len(v.rows); distance++ {
		for _, candidateY := range []int{y + int(direction)*distance, y - int(direction)*distance} {
			if candidateY < 0 || candidateY >= len(v.rows) {
				continue
			}
			if v.rows[candidateY].file != v.rows[y].file {
				continue
			}
			if candidate, ok := v.cursorAt(candidateY, pane); ok {
				return candidate, true
			}
			if v.split {
				if candidate, ok := v.cursorAt(candidateY, pane.other()); ok {
					return candidate, true
				}
			}
		}
	}
	return diffCursor{}, false
}

func (v *diffView) jumpFile(cursor diffCursor, direction direction) (diffCursor, bool) {
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
		if nextFile.OldPath != file.OldPath || nextFile.NewPath != file.NewPath {
			return next, true
		}
		y = next.row
	}
}

func (v *diffView) switchPane(cursor diffCursor, pane diffPane) (diffCursor, bool) {
	if !v.split || !v.valid(cursor) {
		return diffCursor{}, false
	}
	if candidate, ok := v.cursorAt(cursor.row, pane); ok {
		return candidate, true
	}
	for y := cursor.row - 1; y >= 0; y-- {
		if v.rows[y].file != v.rows[cursor.row].file {
			break
		}
		if candidate, ok := v.cursorAt(y, pane); ok {
			return candidate, true
		}
	}
	for y := cursor.row + 1; y < len(v.rows); y++ {
		if candidate, ok := v.cursorAt(y, pane); ok {
			return candidate, true
		}
	}
	return diffCursor{}, false
}
