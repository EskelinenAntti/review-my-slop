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
		return v.searchMetadata(query, y, pane, direction)
	}
	panes := []diffPane{pane}
	if v.split {
		panes = append(panes, pane.other())
	}
	for _, candidatePane := range panes {
		candidate, ok := v.cursorAt(y, candidatePane)
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
		return v.cursorNearRow(y, pane, direction)
	}
	return diffCursor{}, false
}
