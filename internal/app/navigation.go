package app

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
)

func (m *model) toggleSideBySide() {
	enabled := !m.review.sideBySide
	m.setSideBySide(enabled)
	if m.saveLayout != nil {
		if err := m.saveLayout(m.review.sideBySide); err != nil {
			m.err = fmt.Errorf("save side-by-side preference: %w", err)
		}
	}
}

func (m *model) setSideBySide(enabled bool) {
	m.review.sideBySide = enabled
	m.configureDiff()
}

func (m model) updateSearch(name string, key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch name {
	case "esc":
		m.review.view.CancelSearch()
		m.mode = modeBrowse
		m.search.query = nil
	case "enter":
		m.review.view.AcceptSearch()
		m.mode = modeBrowse
		m.search.query = nil
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
	m.review.view.PreviewSearch(string(m.search.query))
}
