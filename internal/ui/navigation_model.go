package ui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
)

func (m *Model) move(direction Direction) {
	next, ok := m.layout.view.Move(m.review.cursor, direction)
	if !ok {
		return
	}
	if m.review.selection != nil {
		selection, selectionOK := m.layout.view.ExtendSelection(*m.review.selection, next)
		if !selectionOK {
			return
		}
		m.review.selection = &selection
	}
	m.setCursor(next)
}

func (m *Model) setCursor(cursor Cursor) {
	m.review.cursor = cursor
	m.layout.keepCursorVisible(cursor)
}

func (m *Model) halfPage(direction Direction) {
	viewport, cursor := m.layout.view.ScrollHalfPage(m.layout.viewport, m.review.cursor, direction)
	if m.review.selection != nil {
		selection, ok := m.layout.view.ExtendSelection(*m.review.selection, cursor)
		if !ok {
			return
		}
		m.review.selection = &selection
	}
	m.layout.viewport, m.review.cursor = viewport, cursor
}

func (m *Model) jumpFile(direction Direction) {
	m.cancelSelection()
	if cursor, ok := m.layout.view.JumpFile(m.review.cursor, direction); ok {
		m.setCursor(cursor)
	}
}

func (m *Model) switchPane(pane Pane) {
	if !m.layout.sideBySideActive() {
		return
	}
	cursor, ok := m.layout.view.SwitchPane(m.review.cursor, pane)
	if !ok {
		return
	}
	if m.review.selection != nil {
		first, firstOK := m.layout.view.SwitchPane(m.review.selection.First, pane)
		last, lastOK := m.layout.view.SwitchPane(m.review.selection.Last, pane)
		if !firstOK || !lastOK {
			return
		}
		selection := m.layout.view.BeginSelection(first)
		selection, ok = m.layout.view.ExtendSelection(selection, last)
		if !ok {
			return
		}
		m.review.selection = &selection
	}
	m.setCursor(cursor)
}

func (m *Model) toggleSideBySide() {
	enabled := !m.layout.sideBySide
	if enabled && m.layout.size.Width < minimumSideBySideWidth {
		m.err = fmt.Errorf("side-by-side view requires a terminal at least %d columns wide", minimumSideBySideWidth)
		return
	}
	m.setSideBySide(enabled)
	if m.dependencies.SaveSideBySide != nil {
		if err := m.dependencies.SaveSideBySide(m.layout.sideBySide); err != nil {
			m.err = fmt.Errorf("save side-by-side preference: %w", err)
		}
	}
}

func (m *Model) setSideBySide(enabled bool) {
	if m.layout.setSideBySide(enabled) {
		m.rebuildReviewView(m.review.changes)
	}
}

func (m Model) updateSearch(name string, key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
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

func (m *Model) updateIncrementalSearch() {
	if len(m.search.query) == 0 {
		m.setCursor(m.search.from)
		m.search.miss = false
		return
	}
	match, ok := m.layout.view.Search(string(m.search.query), m.search.from, Forward)
	m.search.miss = !ok
	if ok {
		m.setCursor(match)
	}
}

func (m *Model) repeatSearch(direction Direction) {
	if m.search.term == "" {
		return
	}
	match, ok := m.layout.view.Search(m.search.term, m.review.cursor, direction)
	if !ok {
		m.err = fmt.Errorf("no matches for %q", m.search.term)
		return
	}
	m.setCursor(match)
}
