package ui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
)

func (m *Model) move(direction Direction) {
	review := &m.review
	next, ok := review.view.Move(review.cursor, direction)
	if !ok {
		return
	}
	if review.selection != nil {
		selection, selectionOK := review.view.ExtendSelection(*review.selection, next)
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
	viewport, cursor := review.view.ScrollHalfPage(review.viewport, review.cursor, direction)
	if review.selection != nil {
		selection, ok := review.view.ExtendSelection(*review.selection, cursor)
		if !ok {
			return
		}
		review.selection = &selection
	}
	review.viewport, review.cursor = viewport, cursor
}

func (m *Model) jumpFile(direction Direction) {
	m.cancelSelection()
	review := &m.review
	if cursor, ok := review.view.JumpFile(review.cursor, direction); ok {
		m.setCursor(cursor)
	}
}

func (m *Model) switchPane(pane Pane) {
	review := &m.review
	if !m.sideBySideActive() {
		return
	}
	cursor, ok := review.view.SwitchPane(review.cursor, pane)
	if !ok {
		return
	}
	if review.selection != nil {
		first, firstOK := review.view.SwitchPane(review.selection.First, pane)
		last, lastOK := review.view.SwitchPane(review.selection.Last, pane)
		if !firstOK || !lastOK {
			return
		}
		selection := review.view.BeginSelection(first)
		selection, ok = review.view.ExtendSelection(selection, last)
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
	enabled := !review.sideBySide
	if enabled && m.width < minimumSideBySideWidth {
		m.err = fmt.Errorf("side-by-side view requires a terminal at least %d columns wide", minimumSideBySideWidth)
		return
	}
	m.setSideBySide(enabled)
	if m.saveLayout != nil {
		if err := m.saveLayout(review.sideBySide); err != nil {
			m.err = fmt.Errorf("save side-by-side preference: %w", err)
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

func (m Model) updateSearch(name string, key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	search := &m.search
	switch name {
	case "esc":
		m.setCursor(search.from)
		m.mode = modeBrowse
		search.query = nil
		search.miss = false
	case "enter":
		if len(search.query) > 0 && !search.miss {
			search.term = string(search.query)
		}
		m.mode = modeBrowse
		search.query = nil
		search.miss = false
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
	search := &m.search
	review := &m.review
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
	search := &m.search
	review := &m.review
	if search.term == "" {
		return
	}
	match, ok := review.view.Search(search.term, review.cursor, direction)
	if !ok {
		m.err = fmt.Errorf("no matches for %q", search.term)
		return
	}
	m.setCursor(match)
}
