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
	for candidate := range v.cursorsAt(y, pane) {
		line, _ := v.line(candidate)
		if strings.Contains(strings.ToLower(line.Text), query) {
			return candidate, true
		}
	}
	return diffCursor{}, false
}

func (v *diffView) cursorNearMetadata(
	y int,
	pane diffPane,
	direction Direction,
) (diffCursor, bool) {
	fileIndex := v.rows[y].file
	for distance := 1; distance <= len(v.rows); distance++ {
		inFile := false
		for _, candidateY := range neighborRows(y, distance, direction) {
			if candidateY < 0 || candidateY >= len(v.rows) ||
				v.rows[candidateY].file != fileIndex {
				continue
			}
			inFile = true
			for candidate := range v.cursorsAt(candidateY, pane) {
				return candidate, true
			}
		}
		// File rows are contiguous, so neither side can re-enter this file.
		if !inFile {
			break
		}
	}
	return diffCursor{}, false
}
