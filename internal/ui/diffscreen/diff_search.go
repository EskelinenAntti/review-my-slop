package diffscreen

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

func (v *diffView) search(
	query string,
	cursor diffCursor,
	direction direction,
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
	direction direction,
) (diffCursor, bool) {
	if v.rows[y].kind != lineRow {
		text := strings.ToLower(ansi.Strip(v.rows[y].text))
		if strings.Contains(text, query) {
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
