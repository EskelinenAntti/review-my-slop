package app

import "fmt"

func (m *model) toggleSideBySide() {
	enabled := !m.patchViewOptions.SideBySide
	m.setSideBySide(enabled)
	if m.saveLayout != nil {
		if err := m.saveLayout(m.patchViewOptions.SideBySide); err != nil {
			m.err = fmt.Errorf("save side-by-side preference: %w", err)
		}
	}
}

func (m *model) setSideBySide(enabled bool) {
	m.patchViewOptions.SideBySide = enabled
	m.patchView.Configure(m.patchViewOptions)
}

func (m *model) configureSideBySide(enabled bool, save saveSideBySideFunc) {
	m.saveLayout = save
	m.setSideBySide(enabled)
}

func (m *model) resizeScreens() {
	m.patchView.Resize(m.width, m.height)
	m.commentView.Resize(m.width, m.height)
}
