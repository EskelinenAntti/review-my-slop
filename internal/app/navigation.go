package app

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/eskelinenantti/review-my-slop/internal/ui/diffscreen"
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
		if key.Text != "" {
			m.diffView.InsertSearch(key.Text)
		}
	}
	return m, nil
}

// navigate applies a diff navigation binding and reports whether it handled the key.
func (m *model) navigate(name string) bool {
	switch name {
	case "n":
		m.diffView.Find(diffscreen.Forward)
	case "N":
		m.diffView.Find(diffscreen.Backward)
	case "j", "down":
		m.diffView.Move(diffscreen.NextLine)
	case "k", "up":
		m.diffView.Move(diffscreen.PreviousLine)
	case "h", "left":
		m.diffView.ScrollHorizontal(-horizontalScrollStep)
	case "l", "right":
		m.diffView.ScrollHorizontal(horizontalScrollStep)
	case "0":
		m.diffView.ScrollHorizontal(-int(^uint(0) >> 1))
	case "$":
		m.diffView.ScrollHorizontal(int(^uint(0) >> 1))
	case "ctrl+d":
		m.diffView.Move(diffscreen.NextPage)
	case "ctrl+u":
		m.diffView.Move(diffscreen.PreviousPage)
	case "g g":
		m.diffView.Move(diffscreen.FirstLine)
	case "G":
		m.diffView.Move(diffscreen.LastLine)
	case "] f":
		m.diffView.Move(diffscreen.NextFile)
	case "[ f":
		m.diffView.Move(diffscreen.PreviousFile)
	case "z z":
		m.diffView.Align(diffscreen.Center)
	case "z t":
		m.diffView.Align(diffscreen.Top)
	case "z b":
		m.diffView.Align(diffscreen.Bottom)
	case "ctrl+w h":
		m.diffView.Move(diffscreen.OldPane)
	case "ctrl+w l":
		m.diffView.Move(diffscreen.NewPane)
	case "ctrl+w ctrl+w":
		m.diffView.Move(diffscreen.OtherPane)
	default:
		return false
	}
	return true
}
