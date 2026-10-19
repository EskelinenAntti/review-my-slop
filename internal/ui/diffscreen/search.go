package diffscreen

// Direction chooses the order in which search matches are visited.
type Direction int8

// Backward and Forward select the search traversal direction.
const (
	Backward Direction = -1
	Forward  Direction = 1
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
