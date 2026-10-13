package app

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
)

func (m model) requestCommentReload() tea.Cmd {
	if m.reloadComments == nil {
		return nil
	}
	revision := m.comments.revision
	return func() tea.Msg {
		loadedComments, err := m.reloadComments()
		return commentsLoadedMsg{
			comments: loadedComments,
			revision: revision,
			err:      err,
		}
	}
}

func (m *model) applyCommentReload(msg commentsLoadedMsg) {
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
