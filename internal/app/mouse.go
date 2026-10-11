package app

import (
	tea "charm.land/bubbletea/v2"
)

func (m *model) updateMouse(message tea.Msg) {
	switch msg := message.(type) {
	case tea.MouseClickMsg:
		m.clickMouse(msg)
	case tea.MouseMotionMsg:
		if msg.Button == tea.MouseLeft && m.mode == modeBrowse {
			m.diffView.DragTo(msg.X, msg.Y)
		}
	case tea.MouseReleaseMsg:
		m.releaseMouse(msg)
	case tea.MouseWheelMsg:
		m.scrollMouse(msg)
	default:
		// Other messages require no mouse action.
	}
}

func (m *model) clickMouse(msg tea.MouseClickMsg) {
	if msg.Button != tea.MouseLeft {
		return
	}
	switch m.mode {
	case modeBrowse:
		m.diffView.BeginDrag(msg.X, msg.Y)
	case modeComments:
		m.commentView.Click(msg.X, msg.Y)
	default:
		// Other modes consume clicks without changing focus.
	}
}

func (m *model) releaseMouse(msg tea.MouseReleaseMsg) {
	if msg.Button != tea.MouseLeft && msg.Button != tea.MouseNone {
		return
	}
	if m.mode == modeBrowse {
		m.diffView.DragTo(msg.X, msg.Y)
	}
	m.diffView.EndDrag()
}

func (m *model) scrollMouse(msg tea.MouseWheelMsg) {
	m.diffView.EndDrag()
	if m.mode != modeBrowse && m.mode != modeSearch {
		return
	}
	switch msg.Button {
	case tea.MouseWheelUp:
		m.diffView.ScrollVertical(-verticalScrollStep)
	case tea.MouseWheelDown:
		m.diffView.ScrollVertical(verticalScrollStep)
	case tea.MouseWheelLeft:
		m.diffView.ScrollHorizontal(-horizontalScrollStep)
	case tea.MouseWheelRight:
		m.diffView.ScrollHorizontal(horizontalScrollStep)
	default:
		// Buttons without a wheel direction do not scroll.
	}
}
