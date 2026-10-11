package app

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
)

func (m model) updateAsync(message tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := message.(type) {
	case commentEditorFinishedMsg:
		m.completeCommentEditor(msg)
	case commentsLoadedMsg:
		m.completeCommentsLoad(msg)
	case sourceEditorFinishedMsg:
		if msg.err != nil {
			m.err = fmt.Errorf("editor: %w", msg.err)
			return m, nil
		}
		return m, m.loadRefresh()
	case tea.FocusMsg:
		return m, m.loadRefresh()
	case refreshDiffMsg:
		m.completeDiffRefresh(msg)
	default:
		m.updateMouse(message)
	}
	return m, nil
}

func (m *model) completeCommentEditor(msg commentEditorFinishedMsg) {
	if msg.err != nil {
		m.err = msg.err
		m.clearCommentEdit()
		return
	}
	m.edit.body = msg.body
	m.finishCommentEdit()
}

func (m *model) completeCommentsLoad(msg commentsLoadedMsg) {
	if msg.revision != m.comments.revision {
		return
	}
	if msg.err != nil {
		m.err = fmt.Errorf("refresh comments: %w", msg.err)
		return
	}
	m.comments.items = msg.comments
	m.commentView.Update(m.comments.items)
	m.err = nil
}

func (m *model) completeDiffRefresh(msg refreshDiffMsg) {
	if msg.kind != m.kind {
		return
	}
	if msg.err != nil {
		m.err = fmt.Errorf("refresh diff: %w", msg.err)
		return
	}
	m.updatePatch(msg.patch)
	m.err = nil
}

func (m model) loadRefresh() tea.Cmd {
	if m.refresh == nil {
		return nil
	}
	kind := m.kind
	return func() tea.Msg {
		p, err := m.refresh(kind)
		return refreshDiffMsg{patch: p, kind: kind, err: err}
	}
}

func (m model) loadComments() tea.Cmd {
	if m.load == nil {
		return nil
	}
	revision := m.comments.revision
	return func() tea.Msg {
		comments, err := m.load()
		return commentsLoadedMsg{
			comments: comments,
			revision: revision,
			err:      err,
		}
	}
}
