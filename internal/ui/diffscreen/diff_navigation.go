package diffscreen

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

func (v *projection) hasSourceLine(cursor diffCursor) bool {
	return cursor.row >= 0 && cursor.row < len(v.rows) && v.lineIndex(v.rows[cursor.row], cursor.pane) >= 0
}

func (v *projection) lineIndex(current displayRow, pane diffPane) int {
	if current.kind != lineRow {
		return -1
	}
	if !v.split || pane == right {
		return current.rightLine
	}
	return current.leftLine
}

func (v *projection) cursorAt(y int, pane diffPane) (diffCursor, bool) {
	cursor := diffCursor{row: y, pane: pane}
	return cursor, v.hasSourceLine(cursor)
}

func (v *projection) firstCursor() (diffCursor, bool) {
	if cursor, ok := v.nextSelectableCursor(-1, right, Forward); ok {
		return cursor, true
	}
	return v.nextSelectableCursor(-1, left, Forward)
}

func (v *projection) lastCursor() (diffCursor, bool) {
	if cursor, ok := v.nextSelectableCursor(len(v.rows), right, Backward); ok {
		return cursor, true
	}
	return v.nextSelectableCursor(len(v.rows), left, Backward)
}

func (v *projection) nextSelectableCursor(start int, pane diffPane, direction Direction) (diffCursor, bool) {
	for y := start + int(direction); y >= 0 && y < len(v.rows); y += int(direction) {
		if cursor, ok := v.cursorAt(y, pane); ok {
			return cursor, true
		}
	}
	return diffCursor{}, false
}

func (v *projection) stepCursor(cursor diffCursor, direction Direction) (diffCursor, bool) {
	if !v.hasSourceLine(cursor) {
		return diffCursor{}, false
	}
	return v.nextSelectableCursor(cursor.row, cursor.pane, direction)
}

func (v *projection) findMatch(query string, cursor diffCursor, direction Direction) (diffCursor, bool) {
	if query == "" || !v.hasSourceLine(cursor) {
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
			line := v.sourceAt(candidate).line
			if strings.Contains(strings.ToLower(line.Text), query) {
				return candidate, true
			}
		}
		if current.kind != lineRow && strings.Contains(strings.ToLower(ansi.Strip(current.text)), query) {
			if candidate, ok := v.nearestFileCursor(y, cursor.pane, direction); ok {
				return candidate, true
			}
		}
	}
	return diffCursor{}, false
}

func (v *projection) nearestFileCursor(y int, pane diffPane, direction Direction) (diffCursor, bool) {
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

func (v *projection) jumpFile(cursor diffCursor, direction Direction) (diffCursor, bool) {
	if !v.hasSourceLine(cursor) {
		return diffCursor{}, false
	}
	file := v.sourceAt(cursor).file
	y := cursor.row
	for {
		next, ok := v.nextSelectableCursor(y, cursor.pane, direction)
		if !ok {
			return diffCursor{}, false
		}
		nextFile := v.sourceAt(next).file
		if nextFile.OldPath != file.OldPath || nextFile.NewPath != file.NewPath {
			return next, true
		}
		y = next.row
	}
}

func (v *projection) switchPane(cursor diffCursor, pane diffPane) (diffCursor, bool) {
	if !v.split || !v.hasSourceLine(cursor) {
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
