package app

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
)

func (m *model) toggleSideBySide() {
	enabled := !m.diffOptions.SideBySide
	m.setSideBySide(enabled)
	if m.saveLayout != nil {
		if err := m.saveLayout(m.diffOptions.SideBySide); err != nil {
			m.err = fmt.Errorf("save side-by-side preference: %w", err)
		}
	}
}

func (m *model) setSideBySide(enabled bool) {
	m.diffOptions.SideBySide = enabled
	m.diffView.Configure(m.diffOptions)
}

func (m model) updateSearch(name string, key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch name {
	case "esc":
		m.diffView.CancelSearch()
		m.mode = modeBrowse
	case "enter":
		m.diffView.AcceptSearch()
		m.mode = modeBrowse
	case "backspace":
		m.diffView.BackspaceSearch()
	default:
		m.insertSearch(key.Text)
	}
	return m, nil
}

func (m *model) insertSearch(text string) {
	if text != "" {
		m.diffView.InsertSearch(text)
	}
}
