package diffscreen

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/eskelinenantti/review-my-slop/internal/patch"
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
	return v.searchRows(strings.ToLower(query), cursor, direction)
}

func (v *diffView) searchRows(query string, cursor diffCursor, direction direction) (diffCursor, bool) {
	for count := 1; count < len(v.rows); count++ {
		y := (cursor.row + int(direction)*count + len(v.rows)) % len(v.rows)
		if candidate, ok := v.searchRow(query, y, cursor.pane, direction); ok {
			return candidate, true
		}
	}
	return diffCursor{}, false
}

func (v *diffView) searchRow(query string, y int, pane diffPane, direction direction) (diffCursor, bool) {
	if candidate, ok := v.searchCode(query, y, pane); ok {
		return candidate, true
	}
	current := v.rows[y]
	if current.kind == lineRow || !strings.Contains(strings.ToLower(ansi.Strip(current.text)), query) {
		return diffCursor{}, false
	}
	return v.cursorNearRow(y, pane, direction)
}

func (v *diffView) searchCode(query string, y int, pane diffPane) (diffCursor, bool) {
	panes := []diffPane{pane}
	if v.split {
		panes = append(panes, pane.other())
	}
	for _, pane := range panes {
		candidate := diffCursor{row: y, pane: pane}
		if v.matchesQuery(candidate, query) {
			return candidate, true
		}
	}
	return diffCursor{}, false
}

func (v *diffView) matchesQuery(cursor diffCursor, query string) bool {
	line, ok := v.line(cursor)
	return ok && strings.Contains(strings.ToLower(line.Text), query)
}

func (v *diffView) cursorNearRow(y int, pane diffPane, direction direction) (diffCursor, bool) {
	for distance := 1; distance <= len(v.rows); distance++ {
		if candidate, ok := v.nearbyCursor(y, distance, pane, direction); ok {
			return candidate, true
		}
	}
	return diffCursor{}, false
}

func (v *diffView) nearbyCursor(y, distance int, pane diffPane, direction direction) (diffCursor, bool) {
	for _, candidateY := range []int{y + int(direction)*distance, y - int(direction)*distance} {
		if candidate, ok := v.sameFileCursor(y, candidateY, pane); ok {
			return candidate, true
		}
	}
	return diffCursor{}, false
}

func (v *diffView) sameFileCursor(y, candidateY int, pane diffPane) (diffCursor, bool) {
	if candidateY < 0 || candidateY >= len(v.rows) {
		return diffCursor{}, false
	}
	if v.rows[candidateY].file != v.rows[y].file {
		return diffCursor{}, false
	}
	return v.cursorEitherPane(candidateY, pane)
}

func (v *diffView) cursorEitherPane(y int, pane diffPane) (diffCursor, bool) {
	if candidate, ok := v.cursorAt(y, pane); ok {
		return candidate, true
	}
	if v.split {
		return v.cursorAt(y, pane.other())
	}
	return diffCursor{}, false
}

func (v *diffView) jumpFile(cursor diffCursor, direction direction) (diffCursor, bool) {
	if !v.valid(cursor) {
		return diffCursor{}, false
	}
	return v.scanOtherFile(cursor, direction)
}

func (v *diffView) scanOtherFile(cursor diffCursor, direction direction) (diffCursor, bool) {
	file, _ := v.file(cursor)
	for next, ok := v.scan(cursor.row, cursor.pane, direction); ok; next, ok = v.scan(next.row, cursor.pane, direction) {
		nextFile, _ := v.file(next)
		if differentPaths(nextFile, file) {
			return next, true
		}
	}
	return diffCursor{}, false
}

func (v *diffView) switchPane(cursor diffCursor, pane diffPane) (diffCursor, bool) {
	if !v.split || !v.valid(cursor) {
		return diffCursor{}, false
	}
	if candidate, ok := v.cursorAt(cursor.row, pane); ok {
		return candidate, true
	}
	if candidate, ok := v.scanPaneAbove(cursor.row, pane); ok {
		return candidate, true
	}
	return v.scan(cursor.row, pane, forward)
}

func (v *diffView) scanPaneAbove(row int, pane diffPane) (diffCursor, bool) {
	for y := row - 1; y >= 0 && v.rows[y].file == v.rows[row].file; y-- {
		if candidate, ok := v.cursorAt(y, pane); ok {
			return candidate, true
		}
	}
	return diffCursor{}, false
}

func differentPaths(a, b patch.File) bool { return a.OldPath != b.OldPath || a.NewPath != b.NewPath }
