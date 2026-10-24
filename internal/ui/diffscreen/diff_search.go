package diffscreen

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// search wraps through every row except the origin. Both panes are searched,
// preferring the current pane; metadata matches focus nearby code in that file.
func (v *diffView) search(
	query string,
	cursor diffCursor,
	direction Direction,
) (diffCursor, bool) {
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
	y int,
	pane diffPane,
	direction Direction,
) (diffCursor, bool) {
	if v.rows[y].kind != lineRow {
		text := ansi.Strip(v.rows[y].text)
		if strings.Contains(strings.ToLower(text), query) {
			return v.cursorNearMetadata(y, pane, direction)
		}
		return diffCursor{}, false
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

func (v *diffView) cursorNearMetadata(
	y int,
	pane diffPane,
	direction Direction,
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
