package app

import (
	tea "charm.land/bubbletea/v2"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

func (m model) updateKey(key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	name := key.String()
	if m.mode == modeComments {
		return m.handleCommentModeKey(name)
	}
	if m.mode == modeHelp {
		if name == "esc" || name == "?" || name == "q" {
			m.mode = modeBrowse
		}
		return m, nil
	}
	if m.mode == modeSearch {
		return m.updateSearch(name, key)
	}
	return m.updateBrowseKey(name)
}

func (m model) updateBrowseKey(name string) (tea.Model, tea.Cmd) {
	m.err = nil
	name = m.keys.Feed(name)
	if m.applyNavigationKey(name) {
		return m, nil
	}
	switch name {
	case "ctrl+c", "q":
		m.quitting = true
		return m, tea.Quit
	case "c":
		return m.startCommentEditor()
	case "e":
		return m.startSourceEditor()
	case "R":
		return m, m.requestPatchRefresh()
	case "tab":
		return m.switchPatchKind()
	default:
		return m.updateBrowseView(name)
	}
}

func (m model) updateBrowseView(name string) (tea.Model, tea.Cmd) {
	switch name {
	case "?":
		m.mode = modeHelp
	case "/":
		m.mode = modeSearch
		m.patchView.BeginSearch()
	case "v":
		m.patchView.ToggleSelection()
	case "esc":
		m.patchView.ClearSelection()
	case "C":
		m.mode = modeComments
		m.commentView.Update(m.comments.items)
		return m, m.requestCommentReload()
	case "t":
		m.toggleSideBySide()
	default:
		return m, nil
	}
	return m, nil
}

func (m model) startCommentEditor() (tea.Model, tea.Cmd) {
	cmd, err := m.startNewComment()
	if err != nil {
		m.err = err
		return m, nil
	}
	return m, cmd
}

func (m model) startSourceEditor() (tea.Model, tea.Cmd) {
	cmd, err := m.openFocusedSourceLine()
	if err != nil {
		m.err = err
		return m, nil
	}
	return m, cmd
}

func (m model) switchPatchKind() (tea.Model, tea.Cmd) {
	if m.currentPatch.Branch == "" {
		return m, nil
	}
	if m.kind == patch.Unstaged {
		m.kind = patch.Branch
	} else {
		m.kind = patch.Unstaged
	}
	m.patchView.ClearSelection()
	return m, m.requestPatchRefresh()
}
