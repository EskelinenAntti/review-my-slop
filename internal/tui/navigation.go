package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/eskelinenantti/review-my-slop/internal/view"
)

func (m *Model) move(direction view.Direction) {
	next, ok := m.view.Move(m.cursor, direction)
	if !ok {
		return
	}
	if m.selection != nil {
		selection, selectionOK := m.view.ExtendSelection(*m.selection, next)
		if !selectionOK {
			return
		}
		m.selection = &selection
	}
	m.setCursor(next)
}

func (m *Model) setCursor(cursor view.Cursor) {
	m.cursor = cursor
	m.viewport = m.view.KeepVisible(m.viewport, cursor)
}

func (m *Model) halfPage(direction view.Direction) {
	viewport, cursor := m.view.ScrollHalfPage(m.viewport, m.cursor, direction)
	if m.selection != nil {
		selection, ok := m.view.ExtendSelection(*m.selection, cursor)
		if !ok {
			return
		}
		m.selection = &selection
	}
	m.viewport, m.cursor = viewport, cursor
}

func (m *Model) jumpFile(direction view.Direction) {
	m.cancelSelection()
	if cursor, ok := m.view.JumpFile(m.cursor, direction); ok {
		m.setCursor(cursor)
	}
}

func (m *Model) switchPane(pane view.Pane) {
	if !m.sideBySideActive() {
		return
	}
	cursor, ok := m.view.SwitchPane(m.cursor, pane)
	if !ok {
		return
	}
	if m.selection != nil {
		first, firstOK := m.view.SwitchPane(m.selection.First, pane)
		last, lastOK := m.view.SwitchPane(m.selection.Last, pane)
		if !firstOK || !lastOK {
			return
		}
		selection := m.view.BeginSelection(first)
		selection, ok = m.view.ExtendSelection(selection, last)
		if !ok {
			return
		}
		m.selection = &selection
	}
	m.setCursor(cursor)
}

func (m Model) sideBySideActive() bool {
	return m.sideBySide && m.width >= minimumSideBySideWidth
}

func (m *Model) toggleSideBySide() {
	enabled := !m.sideBySide
	if enabled && m.width < minimumSideBySideWidth {
		m.err = fmt.Errorf("side-by-side view requires a terminal at least %d columns wide", minimumSideBySideWidth)
		return
	}
	m.setSideBySide(enabled)
	if m.saveLayout != nil {
		if err := m.saveLayout(m.sideBySide); err != nil {
			m.err = fmt.Errorf("save side-by-side preference: %w", err)
		}
	}
}

func (m *Model) setSideBySide(enabled bool) {
	wasActive := m.sideBySideActive()
	m.sideBySide = enabled
	if wasActive != m.sideBySideActive() {
		m.rebuildView(m.patch)
	}
}

func (m Model) updateSearch(name string, key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch name {
	case "esc":
		m.setCursor(m.searchFrom)
		m.mode = modeBrowse
		m.search = nil
		m.searchMiss = false
	case "enter":
		if len(m.search) > 0 && !m.searchMiss {
			m.searchTerm = string(m.search)
		}
		m.mode = modeBrowse
		m.search = nil
		m.searchMiss = false
	case "backspace":
		if len(m.search) > 0 {
			m.search = m.search[:len(m.search)-1]
		}
		m.updateIncrementalSearch()
	default:
		if key.Text != "" {
			m.search = append(m.search, []rune(key.Text)...)
			m.updateIncrementalSearch()
		}
	}
	return m, nil
}

func (m *Model) updateIncrementalSearch() {
	if len(m.search) == 0 {
		m.setCursor(m.searchFrom)
		m.searchMiss = false
		return
	}
	match, ok := m.view.Search(string(m.search), m.searchFrom, view.Forward)
	m.searchMiss = !ok
	if ok {
		m.setCursor(match)
	}
}

func (m *Model) repeatSearch(direction view.Direction) {
	if m.searchTerm == "" {
		return
	}
	match, ok := m.view.Search(m.searchTerm, m.cursor, direction)
	if !ok {
		m.err = fmt.Errorf("no matches for %q", m.searchTerm)
		return
	}
	m.setCursor(match)
}
