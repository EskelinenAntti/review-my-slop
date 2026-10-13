package app

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
)

func (model) Init() tea.Cmd {
	return func() tea.Msg { return tea.RequestBackgroundColor() }
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.BackgroundColorMsg:
		if dark := msg.IsDark(); dark != m.patchViewOptions.Dark {
			m.patchViewOptions.Dark = dark
			m.patchView.Configure(m.patchViewOptions)
		}
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.resizeScreens()

	case tea.KeyPressMsg:
		m.patchView.EndDrag()
		return m.updateKey(msg)
	case tea.MouseClickMsg, tea.MouseMotionMsg,
		tea.MouseReleaseMsg, tea.MouseWheelMsg:
		m.handleMouseMessage(msg)
	default:
		return m.updateWorkflow(msg)
	}
	return m, nil
}

func (m model) updateWorkflow(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case commentEditorFinishedMsg:
		m.completeCommentEditor(msg)
	case commentsLoadedMsg:
		m.applyCommentReload(msg)
	case sourceEditorFinishedMsg:
		if msg.err != nil {
			m.err = fmt.Errorf("editor: %w", msg.err)
			break
		}
		return m, m.requestPatchRefresh()
	case tea.FocusMsg:
		return m, m.requestPatchRefresh()
	case patchRefreshMsg:
		m.applyPatchRefresh(msg)
	default:
		return m, nil
	}
	return m, nil
}
