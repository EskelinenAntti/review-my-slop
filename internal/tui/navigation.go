package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/eskelinenantti/review-my-slop/internal/view"
)

func (m *Model) move(direction view.Direction) {
	next, ok := m.review.view.Move(m.review.cursor, direction)
	if !ok {
		return
	}
	if m.review.selection != nil {
		selection, selectionOK := m.review.view.ExtendSelection(*m.review.selection, next)
		if !selectionOK {
			return
		}
		m.review.selection = &selection
	}
	m.setCursor(next)
}

func (m *Model) setCursor(cursor view.Cursor) {
	m.review.cursor = cursor
	m.review.viewport = m.review.view.KeepVisible(m.review.viewport, cursor)
}

func (m *Model) halfPage(direction view.Direction) {
	viewport, cursor := m.review.view.ScrollHalfPage(m.review.viewport, m.review.cursor, direction)
	if m.review.selection != nil {
		selection, ok := m.review.view.ExtendSelection(*m.review.selection, cursor)
		if !ok {
			return
		}
		m.review.selection = &selection
	}
	m.review.viewport, m.review.cursor = viewport, cursor
}

func (m *Model) jumpFile(direction view.Direction) {
	m.cancelSelection()
	if cursor, ok := m.review.view.JumpFile(m.review.cursor, direction); ok {
		m.setCursor(cursor)
	}
}

func (m *Model) switchPane(pane view.Pane) {
	if !m.sideBySideActive() {
		return
	}
	cursor, ok := m.review.view.SwitchPane(m.review.cursor, pane)
	if !ok {
		return
	}
	if m.review.selection != nil {
		first, firstOK := m.review.view.SwitchPane(m.review.selection.First, pane)
		last, lastOK := m.review.view.SwitchPane(m.review.selection.Last, pane)
		if !firstOK || !lastOK {
			return
		}
		selection := m.review.view.BeginSelection(first)
		selection, ok = m.review.view.ExtendSelection(selection, last)
		if !ok {
			return
		}
		m.review.selection = &selection
	}
	m.setCursor(cursor)
}

func (m Model) sideBySideActive() bool {
	return m.review.sideBySide && m.width >= minimumSideBySideWidth
}

func (m *Model) toggleSideBySide() {
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

func (m *Model) setSideBySide(enabled bool) {
	wasActive := m.sideBySideActive()
	m.review.sideBySide = enabled
	if wasActive != m.sideBySideActive() {
		m.rebuildView(m.review.patch)
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
	match, ok := m.review.view.Search(string(m.search.query), m.search.from, view.Forward)
	m.search.miss = !ok
	if ok {
		m.setCursor(match)
	}
}

func (m *Model) repeatSearch(direction view.Direction) {
	if m.search.term == "" {
		return
	}
	match, ok := m.review.view.Search(m.search.term, m.review.cursor, direction)
	if !ok {
		m.err = fmt.Errorf("no matches for %q", m.search.term)
		return
	}
	m.setCursor(match)
}
