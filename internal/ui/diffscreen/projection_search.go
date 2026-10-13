package diffscreen

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

func (v *patchProjection) searchFromCursor(
	query string,
	cursor patchCursor,
	direction direction,
) (patchCursor, bool) {
	if query == "" || !v.validCursor(cursor) {
		return patchCursor{}, false
	}
	query = strings.ToLower(query)
	y := cursor.row
	for count := 0; count < len(v.rows)-1; count++ {
		y = (y + int(direction) + len(v.rows)) % len(v.rows)
		candidate, ok := v.searchDisplayRow(
			query,
			y,
			cursor.pane,
			direction,
		)
		if ok {
			return candidate, true
		}
	}
	return patchCursor{}, false
}

func (v *patchProjection) searchDisplayRow(
	query string,
	y int,
	pane patchPane,
	direction direction,
) (patchCursor, bool) {
	if v.rows[y].kind != lineRow {
		return v.searchNonCodeRow(query, y, pane, direction)
	}
	panes := []patchPane{pane}
	if v.split {
		panes = append(panes, pane.other())
	}
	for _, candidatePane := range panes {
		candidate, ok := v.cursorOnRow(y, candidatePane)
		if !ok {
			continue
		}
		line, _ := v.lineAtCursor(candidate)
		if strings.Contains(strings.ToLower(line.Text), query) {
			return candidate, true
		}
	}
	return patchCursor{}, false
}

func (v *patchProjection) searchNonCodeRow(
	query string,
	y int,
	pane patchPane,
	direction direction,
) (patchCursor, bool) {
	if strings.Contains(strings.ToLower(ansi.Strip(v.rows[y].text)), query) {
		return v.nearestCursorNearRow(y, pane, direction)
	}
	return patchCursor{}, false
}
