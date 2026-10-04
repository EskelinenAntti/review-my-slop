package ui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
)

func (m *model) move(direction direction) {
	next, ok := m.review.view.move(m.review.cursor, direction)
	if !ok {
		return
	}
	if m.review.selection != nil {
		selection, selectionOK := m.review.view.extendSelection(*m.review.selection, next)
		if !selectionOK {
			return
		}
		m.review.selection = &selection
	}
	m.setCursor(next)
}

func (m *model) setCursor(cursor diffCursor) {
	m.review.cursor = cursor
	m.review.viewport = m.review.view.keepVisible(m.review.viewport, cursor)
}

func (m *model) halfPage(direction direction) {
	viewport, cursor := m.review.view.scrollHalfPage(m.review.viewport, m.review.cursor, direction)
	if m.review.selection != nil {
		selection, ok := m.review.view.extendSelection(*m.review.selection, cursor)
		if !ok {
			return
		}
		m.review.selection = &selection
	}
	m.review.viewport, m.review.cursor = viewport, cursor
}

func (m *model) jumpFile(direction direction) {
	m.cancelSelection()
	if cursor, ok := m.review.view.jumpFile(m.review.cursor, direction); ok {
		m.setCursor(cursor)
	}
}

func (m *model) switchPane(pane diffPane) {
	if !m.sideBySideActive() {
		return
	}
	cursor, ok := m.review.view.switchPane(m.review.cursor, pane)
	if !ok {
		return
	}
	if m.review.selection != nil {
		first, firstOK := m.review.view.switchPane(m.review.selection.First, pane)
		last, lastOK := m.review.view.switchPane(m.review.selection.Last, pane)
		if !firstOK || !lastOK {
			return
		}
		selection := m.review.view.beginSelection(first)
		selection, ok = m.review.view.extendSelection(selection, last)
		if !ok {
			return
		}
		m.review.selection = &selection
	}
	m.setCursor(cursor)
}

func (m model) sideBySideActive() bool {
	return m.review.sideBySide && m.width >= minimumSideBySideWidth
}

func (m *model) toggleSideBySide() {
	enabled := !m.review.sideBySide
	if enabled && m.width < minimumSideBySideWidth {
		m.err = fmt.Errorf("side-by-side view requires a terminal at least %d columns wide", minimumSideBySideWidth)
		return
	}
	m.setSideBySide(enabled)
	if m.saveLayout != nil {
		if err := m.saveLayout(m.review.sideBySide); err != nil {
			m.err = fmt.Errorf("save side-by-side preference: %w", err)
		}
	}
}

func (m *model) setSideBySide(enabled bool) {
	wasActive := m.sideBySideActive()
	m.review.sideBySide = enabled
	if wasActive != m.sideBySideActive() {
		m.rebuildView(m.review.patch)
	}
}

func (m model) updateSearch(name string, key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch name {
	case "esc":
		m.setCursor(m.search.from)
		m.mode = modeBrowse
		m.search.query = nil
		m.search.miss = false
	case "enter":
		if len(m.search.query) > 0 && !m.search.miss {
			m.search.term = string(m.search.query)
		}
		m.mode = modeBrowse
		m.search.query = nil
		m.search.miss = false
	case "backspace":
		if len(m.search.query) > 0 {
			m.search.query = m.search.query[:len(m.search.query)-1]
		}
		m.updateIncrementalSearch()
	default:
		if key.Text != "" {
			m.search.query = append(m.search.query, []rune(key.Text)...)
			m.updateIncrementalSearch()
		}
	}
	return m, nil
}

func (m *model) updateIncrementalSearch() {
	if len(m.search.query) == 0 {
		m.setCursor(m.search.from)
		m.search.miss = false
		return
	}
	match, ok := m.review.view.search(string(m.search.query), m.search.from, forward)
	m.search.miss = !ok
	if ok {
		m.setCursor(match)
	}
}

func (m *model) repeatSearch(direction direction) {
	if m.search.term == "" {
		return
	}
	match, ok := m.review.view.search(m.search.term, m.review.cursor, direction)
	if !ok {
		m.err = fmt.Errorf("no matches for %q", m.search.term)
		return
	}
	m.setCursor(match)
}
