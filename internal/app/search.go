package app

import tea "charm.land/bubbletea/v2"

func (m model) updateSearch(
	name string,
	key tea.KeyPressMsg,
) (tea.Model, tea.Cmd) {
	switch name {
	case "esc":
		m.patchView.CancelSearch()
		m.mode = modeBrowse
	case "enter":
		m.patchView.AcceptSearch()
		m.mode = modeBrowse
	case "backspace":
		m.patchView.BackspaceSearch()
	default:
		if key.Text != "" {
			m.patchView.InsertSearch(key.Text)
		}
	}
	return m, nil
}
