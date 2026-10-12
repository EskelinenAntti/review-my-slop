package app

import tea "charm.land/bubbletea/v2"

func (m *model) updateMouse(msg tea.Msg) {
	switch msg := msg.(type) {
	case tea.MouseClickMsg:
		if msg.Button == tea.MouseLeft {
			switch m.mode {
			case modeBrowse:
				m.diffView.BeginDrag(msg.X, msg.Y)
			case modeComments:
				m.commentView.Click(msg.X, msg.Y)
			}
		}
	case tea.MouseMotionMsg:
		if msg.Button == tea.MouseLeft && m.mode == modeBrowse {
			m.diffView.DragTo(msg.X, msg.Y)
		}
	case tea.MouseReleaseMsg:
		if msg.Button == tea.MouseLeft || msg.Button == tea.MouseNone {
			if m.mode == modeBrowse {
				m.diffView.DragTo(msg.X, msg.Y)
			}
			m.diffView.EndDrag()
		}
	case tea.MouseWheelMsg:
		m.diffView.EndDrag()
		if m.mode != modeBrowse && m.mode != modeSearch {
			break
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
		}
	}
}
