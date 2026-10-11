package app

import (
	"math"

	tea "charm.land/bubbletea/v2"
	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/ui/diffscreen"
)

func (m model) updateKey(key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	name := key.String()
	switch m.mode {
	case modeComments:
		return m.updateComments(name)
	case modeHelp:
		if name == "esc" || name == "?" || name == "q" {
			m.mode = modeBrowse
		}
		return m, nil
	case modeSearch:
		return m.updateSearch(name, key)
	default:
		m.err = nil
		return m.updateBrowse(m.keys.Feed(name))
	}
}

func (m model) updateBrowse(name string) (tea.Model, tea.Cmd) {
	if motion, ok := browseMotions[name]; ok {
		m.diffView.Move(motion)
		return m, nil
	}
	if alignment, ok := browseAlignments[name]; ok {
		m.diffView.Align(alignment)
		return m, nil
	}
	if columns, ok := browseScrolls[name]; ok {
		m.diffView.ScrollHorizontal(columns)
		return m, nil
	}
	return m.updateBrowseAction(name)
}

func (m model) updateBrowseAction(name string) (tea.Model, tea.Cmd) {
	switch name {
	case "ctrl+c", "q":
		m.quitting = true
		return m, tea.Quit
	case "?":
		m.mode = modeHelp
	case "/":
		m.mode = modeSearch
		m.diffView.BeginSearch()
	case "n":
		m.diffView.Find(diffscreen.Forward)
	case "N":
		m.diffView.Find(diffscreen.Backward)
	case "v":
		m.diffView.ToggleSelection()
	case "esc":
		m.diffView.ClearSelection()
	default:
		return m.updateBrowseCommand(name)
	}
	return m, nil
}

func (m model) updateBrowseCommand(name string) (tea.Model, tea.Cmd) {
	switch name {
	case "c":
		cmd, err := m.beginComment()
		m.err = err
		return m, cmd
	case "e":
		cmd, err := m.openCurrentLine()
		m.err = err
		return m, cmd
	case "C":
		m.mode = modeComments
		m.commentView.Update(m.comments.items)
		return m, m.loadComments()
	case "R":
		return m, m.loadRefresh()
	case "tab":
		return m.changePatchKind()
	case "t":
		m.toggleSideBySide()
	default:
		// Unbound keys have no action.
	}
	return m, nil
}

func (m model) changePatchKind() (tea.Model, tea.Cmd) {
	if m.currentPatch.Branch == "" {
		return m, nil
	}
	if m.kind == patch.Unstaged {
		m.kind = patch.Branch
	} else {
		m.kind = patch.Unstaged
	}
	m.diffView.ClearSelection()
	return m, m.loadRefresh()
}

var browseMotions = map[string]diffscreen.Motion{
	"j": diffscreen.NextLine, "down": diffscreen.NextLine,
	"k": diffscreen.PreviousLine, "up": diffscreen.PreviousLine,
	"ctrl+d": diffscreen.NextPage, "ctrl+u": diffscreen.PreviousPage,
	"g g": diffscreen.FirstLine, "G": diffscreen.LastLine,
	"] f": diffscreen.NextFile, "[ f": diffscreen.PreviousFile,
	"ctrl+w h": diffscreen.OldPane, "ctrl+w l": diffscreen.NewPane,
	"ctrl+w ctrl+w": diffscreen.OtherPane,
}

var browseAlignments = map[string]diffscreen.Alignment{
	"z z": diffscreen.Center, "z t": diffscreen.Top, "z b": diffscreen.Bottom,
}

var browseScrolls = map[string]int{
	"h": -horizontalScrollStep, "left": -horizontalScrollStep,
	"l": horizontalScrollStep, "right": horizontalScrollStep,
	"0": -math.MaxInt, "$": math.MaxInt,
}
