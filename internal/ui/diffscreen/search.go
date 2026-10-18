package diffscreen

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

type searchState struct {
	active           bool
	query, term      string
	from             diffCursor
	miss, repeatMiss bool
}

// BeginSearch starts an empty search query at the current focus.
func (v *View) BeginSearch() {
	v.ClearSelection()
	v.search.active = true
	v.search.query, v.search.miss = "", false
	v.search.from = v.cursor
}

// InsertSearch appends text to the query and previews from the search origin.
// It starts a search session if needed; a miss retains the last successful
// focus.
func (v *View) InsertSearch(text string) {
	if !v.search.active {
		v.BeginSearch()
	}
	v.previewSearch(v.search.query + text)
}

// BackspaceSearch removes the last Unicode code point from the active query.
// An empty query restores the search origin. Outside a search it does nothing.
func (v *View) BackspaceSearch() {
	if !v.search.active {
		return
	}
	query := []rune(v.search.query)
	if len(query) > 0 {
		query = query[:len(query)-1]
	}
	v.previewSearch(string(query))
}

func (v *View) previewSearch(query string) {
	v.search.query = query
	if query == "" {
		v.setCursor(v.search.from)
		v.search.miss = false
		return
	}
	cursor, ok := v.view.search(query, v.search.from, forward)
	v.search.miss = !ok
	if ok {
		v.setCursor(cursor)
	}
}

// AcceptSearch remembers a successful query for subsequent searches.
func (v *View) AcceptSearch() {
	if !v.search.active {
		return
	}
	if v.search.query != "" && !v.search.miss {
		v.search.term = v.search.query
	}
	v.search.active, v.search.miss = false, false
	v.search.query = ""
}

// CancelSearch restores the focus from before the active search.
func (v *View) CancelSearch() {
	if !v.search.active {
		return
	}
	v.setCursor(v.search.from)
	v.search.active, v.search.miss = false, false
	v.search.query = ""
}

// Find moves to the next remembered search match in the requested direction.
func (v *View) Find(d Direction) bool {
	if v.search.term == "" {
		return false
	}
	if d != Forward && d != Backward {
		return false
	}
	cursor, ok := v.view.search(v.search.term, v.cursor, direction(d))
	v.search.repeatMiss = !ok
	if ok {
		v.setCursor(cursor)
	}
	return ok
}

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

func (v *diffView) searchMetadata(
	query string,
	y int,
	pane diffPane,
	direction direction,
) (diffCursor, bool) {
	if strings.Contains(strings.ToLower(ansi.Strip(v.rows[y].text)), query) {
		return v.cursorNearMetadata(y, pane, direction)
	}
	return diffCursor{}, false
}

func (v *diffView) cursorNearMetadata(
	y int,
	pane diffPane,
	direction direction,
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

func neighborRows(row, distance int, direction direction) [2]int {
	offset := int(direction) * distance
	return [2]int{row + offset, row - offset}
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
