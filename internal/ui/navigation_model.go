package ui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
)

func (m *Model) move(direction Direction) {
	next, ok := m.changes.view.Move(m.changes.cursor, direction)
	if !ok {
		return
	}
	if m.changes.selection != nil {
		selection, selectionOK := m.changes.view.ExtendSelection(*m.changes.selection, next)
		if !selectionOK {
			return
		}
		m.changes.selection = &selection
	}
	m.setCursor(next)
}

func (m *Model) setCursor(cursor Cursor) {
	m.changes.cursor = cursor
	m.changes.viewport = m.changes.view.KeepVisible(m.changes.viewport, cursor)
}

func (m *Model) halfPage(direction Direction) {
	viewport, cursor := m.changes.view.ScrollHalfPage(m.changes.viewport, m.changes.cursor, direction)
	if m.changes.selection != nil {
		selection, ok := m.changes.view.ExtendSelection(*m.changes.selection, cursor)
		if !ok {
			return
		}
		m.changes.selection = &selection
	}
	m.changes.viewport, m.changes.cursor = viewport, cursor
}

func (m *Model) jumpFile(direction Direction) {
	m.cancelSelection()
	if cursor, ok := m.changes.view.JumpFile(m.changes.cursor, direction); ok {
		m.setCursor(cursor)
	}
}

func (m *Model) switchPane(pane Pane) {
	if !m.sideBySideActive() {
		return
	}
	cursor, ok := m.changes.view.SwitchPane(m.changes.cursor, pane)
	if !ok {
		return
	}
	if m.changes.selection != nil {
		first, firstOK := m.changes.view.SwitchPane(m.changes.selection.First, pane)
		last, lastOK := m.changes.view.SwitchPane(m.changes.selection.Last, pane)
		if !firstOK || !lastOK {
			return
		}
		selection := m.changes.view.BeginSelection(first)
		selection, ok = m.changes.view.ExtendSelection(selection, last)
		if !ok {
			return
		}
		m.changes.selection = &selection
	}
	m.setCursor(cursor)
}

func (m Model) sideBySideActive() bool {
	return m.changes.sideBySide && m.width >= minimumSideBySideWidth
}

func (m *Model) toggleSideBySide() {
	enabled := !m.changes.sideBySide
	if enabled && m.width < minimumSideBySideWidth {
		m.err = fmt.Errorf("side-by-side view requires a terminal at least %d columns wide", minimumSideBySideWidth)
		return
	}
	m.setSideBySide(enabled)
	if m.dependencies.SaveSideBySide != nil {
		if err := m.dependencies.SaveSideBySide(m.changes.sideBySide); err != nil {
			m.err = fmt.Errorf("save side-by-side preference: %w", err)
		}
	}
}

func (m *Model) setSideBySide(enabled bool) {
	wasActive := m.sideBySideActive()
	m.changes.sideBySide = enabled
	if wasActive != m.sideBySideActive() {
		m.rebuildView(m.changes.changes)
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
	match, ok := m.changes.view.Search(string(m.search.query), m.search.from, Forward)
	m.search.miss = !ok
	if ok {
		m.setCursor(match)
	}
}

func (m *Model) repeatSearch(direction Direction) {
	if m.search.term == "" {
		return
	}
	match, ok := m.changes.view.Search(m.search.term, m.changes.cursor, direction)
	if !ok {
		m.err = fmt.Errorf("no matches for %q", m.search.term)
		return
	}
	m.setCursor(match)
}
