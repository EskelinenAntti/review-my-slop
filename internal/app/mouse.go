package app

import tea "charm.land/bubbletea/v2"

func (m *model) handleMouseMessage(msg tea.Msg) {
	switch msg := msg.(type) {
	case tea.MouseClickMsg:
		m.handleMouseClick(msg)
	case tea.MouseMotionMsg:
		m.updateMouseDrag(msg)
	case tea.MouseReleaseMsg:
		m.finishMouseDrag(msg)
	case tea.MouseWheelMsg:
		m.scrollWithMouseWheel(msg)
	default:
		return
	}
}

func (m *model) handleMouseClick(msg tea.MouseClickMsg) {
	if msg.Button != tea.MouseLeft {
		return
	}
	switch m.mode {
	case modeBrowse:
		m.patchView.BeginDrag(msg.X, msg.Y)
	case modeComments:
		m.commentView.Click(msg.X, msg.Y)
	default:
		return
	}
}

func (m *model) updateMouseDrag(msg tea.MouseMotionMsg) {
	if msg.Button == tea.MouseLeft && m.mode == modeBrowse {
		m.patchView.DragTo(msg.X, msg.Y)
	}
}

func (m *model) finishMouseDrag(msg tea.MouseReleaseMsg) {
	if msg.Button != tea.MouseLeft && msg.Button != tea.MouseNone {
		return
	}
	if m.mode == modeBrowse {
		m.patchView.DragTo(msg.X, msg.Y)
	}
	m.patchView.EndDrag()
}

func (m *model) scrollWithMouseWheel(msg tea.MouseWheelMsg) {
	m.patchView.EndDrag()
	if m.mode != modeBrowse && m.mode != modeSearch {
		return
	}
	switch msg.Button {
	case tea.MouseWheelUp:
		m.patchView.ScrollVertical(-verticalScrollStep)
	case tea.MouseWheelDown:
		m.patchView.ScrollVertical(verticalScrollStep)
	case tea.MouseWheelLeft:
		m.patchView.ScrollHorizontal(-horizontalScrollStep)
	case tea.MouseWheelRight:
		m.patchView.ScrollHorizontal(horizontalScrollStep)
	default:
		return
	}
}
