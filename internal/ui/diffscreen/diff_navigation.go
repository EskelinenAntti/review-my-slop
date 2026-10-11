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
		y = wrappedRow(y+int(direction), len(v.rows))
		if candidate, ok := v.searchRow(query, y, cursor.pane, direction); ok {
			return candidate, true
		}
	}
	return diffCursor{}, false
}

func wrappedRow(y, count int) int {
	if y < 0 {
		return count - 1
	}
	if y >= count {
		return 0
	}
	return y
}

func (v *diffView) searchRow(query string, y int, pane diffPane, direction direction) (diffCursor, bool) {
	if candidate, ok := v.searchCodeRow(query, y, pane); ok {
		return candidate, true
	}
	current := v.rows[y]
	if current.kind != lineRow && strings.Contains(strings.ToLower(ansi.Strip(current.text)), query) {
		return v.cursorNearRow(y, pane, direction)
	}
	return diffCursor{}, false
}

func (v *diffView) searchCodeRow(query string, y int, preferred diffPane) (diffCursor, bool) {
	for _, pane := range v.searchPanes(preferred) {
		candidate, ok := v.cursorAt(y, pane)
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

func (v *diffView) searchPanes(preferred diffPane) []diffPane {
	if v.split {
		return []diffPane{preferred, preferred.other()}
	}
	return []diffPane{preferred}
}

func (v *diffView) cursorNearRow(y int, pane diffPane, direction direction) (diffCursor, bool) {
	for distance := 1; distance <= len(v.rows); distance++ {
		for _, candidateY := range []int{y + int(direction)*distance, y - int(direction)*distance} {
			if candidate, ok := v.cursorInFile(candidateY, y, pane); ok {
				return candidate, true
			}
		}
	}
	return diffCursor{}, false
}

func (v *diffView) cursorInFile(y, origin int, pane diffPane) (diffCursor, bool) {
	if y < 0 || y >= len(v.rows) {
		return diffCursor{}, false
	}
	if v.rows[y].file != v.rows[origin].file {
		return diffCursor{}, false
	}
	return v.cursorInPanes(y, pane)
}

func (v *diffView) cursorInPanes(y int, pane diffPane) (diffCursor, bool) {
	for _, candidatePane := range v.searchPanes(pane) {
		if candidate, ok := v.cursorAt(y, candidatePane); ok {
			return candidate, true
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
		if differentPaths(nextFile.OldPath, nextFile.NewPath, file.OldPath, file.NewPath) {
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
	if candidate, ok := v.previousPaneCursor(cursor, pane); ok {
		return candidate, true
	}
	return v.scan(cursor.row, pane, forward)
}

func (v *diffView) previousPaneCursor(cursor diffCursor, pane diffPane) (diffCursor, bool) {
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

func differentPaths(old, new, targetOld, targetNew string) bool {
	return old != targetOld || new != targetNew
}
