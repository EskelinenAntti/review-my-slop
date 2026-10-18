package app

import (
	"fmt"
	"math"

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

func (m model) updateSearch(
	name string,
	key tea.KeyPressMsg,
) (tea.Model, tea.Cmd) {
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

// navigate applies a diff navigation binding and reports whether it handled the
// key.
func (m *model) navigate(name string) bool {
	if motion, ok := navigationMotions[name]; ok {
		m.diffView.Move(motion)
		return true
	}
	if alignment, ok := navigationAlignments[name]; ok {
		m.diffView.Align(alignment)
		return true
	}
	switch name {
	case "n":
		m.diffView.Find(diffscreen.Forward)
	case "N":
		m.diffView.Find(diffscreen.Backward)
	case "h", "left":
		m.diffView.ScrollHorizontal(-horizontalScrollStep)
	case "l", "right":
		m.diffView.ScrollHorizontal(horizontalScrollStep)
	case "0":
		m.diffView.ScrollHorizontal(-math.MaxInt)
	case "$":
		m.diffView.ScrollHorizontal(math.MaxInt)
	default:
		return false
	}
	return true
}

var navigationMotions = map[string]diffscreen.Motion{
	"j":             diffscreen.NextLine,
	"down":          diffscreen.NextLine,
	"k":             diffscreen.PreviousLine,
	"up":            diffscreen.PreviousLine,
	"ctrl+d":        diffscreen.NextPage,
	"ctrl+u":        diffscreen.PreviousPage,
	"g g":           diffscreen.FirstLine,
	"G":             diffscreen.LastLine,
	"] f":           diffscreen.NextFile,
	"[ f":           diffscreen.PreviousFile,
	"ctrl+w h":      diffscreen.OldPane,
	"ctrl+w l":      diffscreen.NewPane,
	"ctrl+w ctrl+w": diffscreen.OtherPane,
}

var navigationAlignments = map[string]diffscreen.Alignment{
	"z z": diffscreen.Center,
	"z t": diffscreen.Top,
	"z b": diffscreen.Bottom,
}
