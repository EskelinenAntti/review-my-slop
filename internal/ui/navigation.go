package ui

func (m *Model) move(direction Direction) {
	review := &m.review
	view := review.view
	next, ok := view.Move(review.cursor, direction)
	if !ok {
		return
	}
	currentSelection := review.selection
	if currentSelection != nil {
		selection, selectionOK := view.ExtendSelection(*currentSelection, next)
		if !selectionOK {
			return
		}
		review.selection = &selection
	}
	m.setCursor(next)
}

func (m *Model) setCursor(cursor Cursor) {
	review := &m.review
	review.cursor = cursor
	review.viewport = review.view.KeepVisible(review.viewport, cursor)
}

func (m *Model) halfPage(direction Direction) {
	review := &m.review
	view := review.view
	viewport, cursor := view.ScrollHalfPage(review.viewport, review.cursor, direction)
	currentSelection := review.selection
	if currentSelection != nil {
		selection, ok := view.ExtendSelection(*currentSelection, cursor)
		if !ok {
			return
		}
		review.selection = &selection
	}
	review.viewport, review.cursor = viewport, cursor
}

func (m *Model) jumpFile(direction Direction) {
	review := &m.review
	m.cancelSelection()
	if cursor, ok := review.view.JumpFile(review.cursor, direction); ok {
		m.setCursor(cursor)
	}
}

func (m *Model) switchPane(pane Pane) {
	review := &m.review
	if !m.sideBySideActive() {
		return
	}
	view := review.view
	switchViewPane := view.SwitchPane
	cursor, ok := switchViewPane(review.cursor, pane)
	if !ok {
		return
	}
	currentSelection := review.selection
	if currentSelection != nil {
		first, firstOK := switchViewPane(currentSelection.First, pane)
		last, lastOK := switchViewPane(currentSelection.Last, pane)
		if !firstOK || !lastOK {
			return
		}
		selection := view.BeginSelection(first)
		selection, ok = view.ExtendSelection(selection, last)
		if !ok {
			return
		}
		review.selection = &selection
	}
	m.setCursor(cursor)
}

func (m Model) sideBySideActive() bool {
	return m.review.sideBySide && m.width >= minimumSideBySideWidth
}

func (m *Model) toggleSideBySide() {
	review := &m.review
	saveLayout := m.saveLayout
	enabled := !review.sideBySide
	if enabled && m.width < minimumSideBySideWidth {
		m.err = formatError("side-by-side view requires a terminal at least %d columns wide", minimumSideBySideWidth)
		return
	}
	m.setSideBySide(enabled)
	if saveLayout != nil {
		if err := saveLayout(review.sideBySide); err != nil {
			m.err = formatError("save side-by-side preference: %w", err)
		}
	}
}

func (m *Model) setSideBySide(enabled bool) {
	review := &m.review
	wasActive := m.sideBySideActive()
	review.sideBySide = enabled
	if wasActive != m.sideBySideActive() {
		m.rebuildView(review.patch)
	}
}

func (m Model) updateSearch(name string, key teaKeyPress) (teaModel, teaCmd) {
	search := &m.search
	query := search.query
	hasQuery := len(query) > 0
	miss, text := search.miss, key.Text
	updateIncremental := m.updateIncrementalSearch
	update := false
	switch name {
	case "esc":
		m.setCursor(search.from)
		m.mode = modeBrowse
		query = nil
		miss = false
	case "enter":
		if hasQuery && !miss {
			search.term = string(query)
		}
		m.mode = modeBrowse
		query = nil
		miss = false
	case "backspace":
		if hasQuery {
			query = query[:len(query)-1]
		}
		update = true
	default:
		if text != "" {
			query = append(query, []rune(text)...)
			update = true
		}
	}
	search.query = query
	search.miss = miss
	if update {
		updateIncremental(query)
	}
	return m, nil
}

func (m *Model) updateIncrementalSearch(query []rune) {
	review, search := &m.review, &m.search
	from, setCursor := search.from, m.setCursor
	if len(query) == 0 {
		setCursor(from)
		search.miss = false
		return
	}
	match, ok := review.view.Search(string(query), from, Forward)
	search.miss = !ok
	if ok {
		setCursor(match)
	}
}

func (m *Model) repeatSearch(direction Direction) {
	review, search := &m.review, &m.search
	term := search.term
	if term == "" {
		return
	}
	match, ok := review.view.Search(term, review.cursor, direction)
	if !ok {
		m.err = formatError("no matches for %q", term)
		return
	}
	m.setCursor(match)
}
