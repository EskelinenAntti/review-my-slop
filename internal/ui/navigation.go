package ui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
)

func (m *Model) move(direction Direction) {
	if next, ok := m.review.view.Move(m.review.Cursor(), direction); ok {
		m.setCursor(next)
	}
}

// setCursor updates the selection and viewport together. All movement uses the
// active endpoint, and visual movement cannot leave the anchor's file or hunk.
func (m *Model) setCursor(cursor Cursor) bool {
	return m.moveTo(cursor, m.review.Viewport)
}

func (m *Model) moveTo(cursor Cursor, viewport Viewport) bool {
	if _, ok := m.review.view.Line(cursor); !ok {
		return false
	}
	selection := m.review.view.BeginSelection(cursor)
	if m.review.Extending {
		var ok bool
		selection, ok = m.review.view.ExtendSelection(*m.review.Selection, cursor)
		if !ok {
			return false
		}
	}
	m.review.Selection = &selection
	m.review.Viewport = m.review.view.KeepVisible(viewport, cursor)
	return true
}

func (m *Model) halfPage(direction Direction) {
	viewport, cursor := m.review.view.ScrollHalfPage(m.review.Viewport, m.review.Cursor(), direction)
	m.moveTo(cursor, viewport)
}

func (m *Model) jumpFile(direction Direction) {
	m.cancelSelection()
	if cursor, ok := m.review.view.JumpFile(m.review.Cursor(), direction); ok {
		m.setCursor(cursor)
	}
}

func (m *Model) switchPane(pane Pane) {
	if !m.sideBySideActive() {
		return
	}
	cursor, ok := m.review.view.SwitchPane(m.review.Cursor(), pane)
	if !ok {
		return
	}
	if m.review.Extending {
		first, firstOK := m.review.view.SwitchPane(m.review.Selection.First, pane)
		last, lastOK := m.review.view.SwitchPane(m.review.Selection.Last, pane)
		if !firstOK || !lastOK {
			return
		}
		selection := m.review.view.BeginSelection(first)
		selection, ok = m.review.view.ExtendSelection(selection, last)
		if !ok {
			return
		}
		m.review.Selection = &selection
		m.review.Viewport = m.review.view.KeepVisible(m.review.Viewport, selection.Last)
		return
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
	match, ok := m.review.view.Search(string(m.search.query), m.search.from, Forward)
	m.search.miss = !ok
	if ok {
		m.setCursor(match)
	}
}

func (m *Model) repeatSearch(direction Direction) {
	if m.search.term == "" {
		return
	}
	match, ok := m.review.view.Search(m.search.term, m.review.Cursor(), direction)
	if !ok {
		m.err = fmt.Errorf("no matches for %q", m.search.term)
		return
	}
	m.setCursor(match)
}
