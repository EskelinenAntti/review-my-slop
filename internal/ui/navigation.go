package ui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
)

func (m *Model) move(direction Direction) {
	review := &m.review
	if review.view.valid(review.cursor) {
		if next, ok := review.view.scan(review.cursor.Coordinate, review.cursor.Pane, direction); ok {
			if review.selection != nil {
				selection, selectionOK := review.view.ExtendSelection(*review.selection, next)
				if !selectionOK {
					return
				}
				review.selection = &selection
			}
			m.setCursor(next)
		}
	}
}

func (m *Model) setCursor(cursor Cursor) {
	review := &m.review
	review.cursor = cursor
	review.viewport = review.view.KeepVisible(review.viewport, cursor)
}

func (m *Model) halfPage(direction Direction) {
	review := &m.review
	view := review.view
	viewport, cursor := review.viewport, review.cursor
	if view.valid(cursor) {
		distance := int(direction) * max(1, viewport.Height/2)
		viewport.Top += distance
		viewport = view.clampViewport(viewport)
		target := min(len(view.rows)-1, max(0, cursor.Coordinate+distance))
		height, top := view.contentHeight(viewport), viewport.Top
	search:
		for distance := range height {
			offset := int(direction) * distance
			for _, y := range []int{target + offset, target - offset} {
				if y >= top && y < top+height && y < len(view.rows) {
					if candidate, ok := view.cursorAt(y, cursor.Pane); ok {
						cursor = candidate
						break search
					}
				}
			}
		}
	}
	if review.selection != nil {
		selection, ok := view.ExtendSelection(*review.selection, cursor)
		if !ok {
			return
		}
		review.selection = &selection
	}
	review.viewport, review.cursor = viewport, cursor
}

func (m *Model) jumpFile(direction Direction) {
	review := &m.review
	review.selection = nil
	view := review.view
	if view.valid(review.cursor) {
		fileIndex, y := view.rows[review.cursor.Coordinate].file, review.cursor.Coordinate
		for {
			cursor, ok := view.scan(y, review.cursor.Pane, direction)
			if !ok {
				return
			}
			if view.rows[cursor.Coordinate].file != fileIndex {
				m.setCursor(cursor)
				return
			}
			y = cursor.Coordinate
		}
	}
}

func (m *Model) switchPane(pane Pane) {
	review := &m.review
	view := review.view
	switchPane := view.SwitchPane
	if m.sideBySideActive() {
		if cursor, ok := switchPane(review.cursor, pane); ok {
			currentSelection := review.selection
			if currentSelection != nil {
				first, firstOK := switchPane(currentSelection.First, pane)
				last, lastOK := switchPane(currentSelection.Last, pane)
				if !firstOK || !lastOK {
					return
				}
				selection, ok := view.ExtendSelection(Selection{first, first}, last)
				if !ok {
					return
				}
				review.selection = &selection
			}
			m.setCursor(cursor)
		}
	}
}

func (m Model) sideBySideActive() bool {
	return m.review.sideBySide && m.width >= minimumSideBySideWidth
}

func (m *Model) toggleSideBySide() {
	review := &m.review
	enabled := !review.sideBySide
	if enabled && m.width < minimumSideBySideWidth {
		m.err = fmt.Errorf("side-by-side view requires a terminal at least %d columns wide", minimumSideBySideWidth)
		return
	}
	wasActive := m.sideBySideActive()
	review.sideBySide = enabled
	if wasActive != m.sideBySideActive() {
		m.rebuildView(review.patch)
	}
	if m.saveLayout != nil {
		if err := m.saveLayout(review.sideBySide); err != nil {
			m.err = fmt.Errorf("save side-by-side preference: %w", err)
		}
	}
}

func (m Model) updateSearch(name string, key tea.KeyPressMsg) (tea.Model, uiCommand) {
	search := &m.search
	switch name {
	case "esc", "enter":
		if name == "esc" {
			m.setCursor(search.from)
		} else if len(search.query) > 0 && !search.miss {
			search.term = string(search.query)
		}
		m.mode = modeBrowse
		search.query, search.miss = nil, false
	case "backspace":
		if len(search.query) > 0 {
			search.query = search.query[:len(search.query)-1]
		}
		m.updateIncrementalSearch()
	default:
		if key.Text != "" {
			search.query = append(search.query, []rune(key.Text)...)
			m.updateIncrementalSearch()
		}
	}
	return m, nil
}

func (m *Model) updateIncrementalSearch() {
	search, review := &m.search, &m.review
	if len(search.query) == 0 {
		m.setCursor(search.from)
		search.miss = false
		return
	}
	match, ok := review.view.Search(string(search.query), search.from, Forward)
	search.miss = !ok
	if ok {
		m.setCursor(match)
	}
}

func (m *Model) repeatSearch(direction Direction) {
	search, review := &m.search, &m.review
	if search.term != "" {
		match, ok := review.view.Search(search.term, review.cursor, direction)
		if !ok {
			m.err = fmt.Errorf("no matches for %q", search.term)
			return
		}
		m.setCursor(match)
	}
}
