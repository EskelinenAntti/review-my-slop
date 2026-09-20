package ui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
)

func (r *reviewState) move(direction Direction) {
	next, ok := r.view.Move(r.cursor, direction)
	if !ok {
		return
	}
	if r.selection != nil {
		selection, selectionOK := r.view.ExtendSelection(*r.selection, next)
		if !selectionOK {
			return
		}
		r.selection = &selection
	}
	r.setCursor(next)
}

func (r *reviewState) setCursor(cursor Cursor) {
	r.cursor = cursor
	r.viewport = r.view.KeepVisible(r.viewport, cursor)
}

func (r *reviewState) halfPage(direction Direction) {
	viewport, cursor := r.view.ScrollHalfPage(r.viewport, r.cursor, direction)
	if r.selection != nil {
		selection, ok := r.view.ExtendSelection(*r.selection, cursor)
		if !ok {
			return
		}
		r.selection = &selection
	}
	r.viewport, r.cursor = viewport, cursor
}

func (r *reviewState) jumpFile(direction Direction) {
	r.selection = nil
	if cursor, ok := r.view.JumpFile(r.cursor, direction); ok {
		r.setCursor(cursor)
	}
}

func (r *reviewState) switchPane(pane Pane, active bool) {
	if !active {
		return
	}
	cursor, ok := r.view.SwitchPane(r.cursor, pane)
	if !ok {
		return
	}
	if r.selection != nil {
		first, firstOK := r.view.SwitchPane(r.selection.First, pane)
		last, lastOK := r.view.SwitchPane(r.selection.Last, pane)
		if !firstOK || !lastOK {
			return
		}
		selection := r.view.BeginSelection(first)
		selection, ok = r.view.ExtendSelection(selection, last)
		if !ok {
			return
		}
		r.selection = &selection
	}
	r.setCursor(cursor)
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
		m.review.setCursor(m.review.search.from)
		m.review.search.active = false
		m.review.search.query = nil
		m.review.search.miss = false
	case "enter":
		if len(m.review.search.query) > 0 && !m.review.search.miss {
			m.review.search.term = string(m.review.search.query)
		}
		m.review.search.active = false
		m.review.search.query = nil
		m.review.search.miss = false
	case "backspace":
		if len(m.review.search.query) > 0 {
			m.review.search.query = m.review.search.query[:len(m.review.search.query)-1]
		}
		m.updateIncrementalSearch()
	default:
		if key.Text != "" {
			m.review.search.query = append(m.review.search.query, []rune(key.Text)...)
			m.updateIncrementalSearch()
		}
	}
	return m, nil
}

func (m *Model) updateIncrementalSearch() {
	if len(m.review.search.query) == 0 {
		m.review.setCursor(m.review.search.from)
		m.review.search.miss = false
		return
	}
	match, ok := m.review.view.Search(string(m.review.search.query), m.review.search.from, Forward)
	m.review.search.miss = !ok
	if ok {
		m.review.setCursor(match)
	}
}

func (m *Model) repeatSearch(direction Direction) {
	if m.review.search.term == "" {
		return
	}
	match, ok := m.review.view.Search(m.review.search.term, m.review.cursor, direction)
	if !ok {
		m.err = fmt.Errorf("no matches for %q", m.review.search.term)
		return
	}
	m.review.setCursor(match)
}
