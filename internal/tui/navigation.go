package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/eskelinenantti/review-my-slop/internal/view"
)

func (m *Model) navigate(command view.Command) {
	state, outcome := view.Navigate(m.review.view, m.review.state, command)
	m.review.state = state
	if outcome == view.NoMatch {
		m.err = fmt.Errorf("no matches for %q", m.search.term)
	}
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
		if m.search.from != nil {
			m.setCursor(*m.search.from)
		}
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
		if m.search.from != nil {
			m.setCursor(*m.search.from)
		}
		m.search.miss = false
		return
	}
	state := m.review.state
	if m.search.from != nil {
		state.Cursor = m.search.from
	}
	state, outcome := view.Navigate(m.review.view, state, view.Search(string(m.search.query), view.Forward))
	m.search.miss = outcome == view.NoMatch
	if outcome == view.NoOutcome {
		m.review.state = state
	}
}

func (m *Model) repeatSearch(direction view.Direction) {
	if m.search.term == "" {
		return
	}
	m.navigate(view.Search(m.search.term, direction))
}

func (m *Model) setCursor(cursor view.Cursor) {
	m.review.state.Cursor = &cursor
	m.review.state.Viewport = m.review.view.KeepVisible(m.review.state.Viewport, cursor)
}
