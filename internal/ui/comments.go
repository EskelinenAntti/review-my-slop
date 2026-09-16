package ui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/eskelinenantti/review-my-slop/internal/comment"
)

func (m Model) updateComments(name string) (tea.Model, tea.Cmd) {
	m.err = nil
	switch name {
	case "esc", "C", "q":
		m.mode = modeBrowse
	case "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	case "j", "down":
		if m.comments.row < len(m.comments.items)-1 {
			m.comments.row++
		}
	case "k", "up":
		if m.comments.row > 0 {
			m.comments.row--
		}
	case "enter", "e":
		if len(m.comments.items) > 0 {
			m.comments.editIndex = m.comments.row
			m.comments.body = m.comments.items[m.comments.editIndex].Body
			m.comments.editAnchor = m.comments.items[m.comments.editIndex].Anchor
			cmd, err := m.openCommentEditor()
			if err != nil {
				m.err = err
				m.clearCommentEdit()
				return m, nil
			}
			return m, cmd
		}
	case "D":
		if len(m.comments.items) > 0 {
			m.deleteComment(m.comments.row)
		}
	}
	return m, nil
}

func (m *Model) beginComment() (tea.Cmd, error) {
	selection := m.review.selection
	if selection == nil {
		current := m.layout.view.BeginSelection(m.review.cursor)
		selection = &current
	}
	anchor, err := m.layout.view.Anchor(*selection)
	if err != nil {
		return nil, err
	}
	m.comments.body, m.comments.editIndex, m.comments.editAnchor = "", -1, anchor
	cmd, err := m.openCommentEditor()
	if err != nil {
		m.clearCommentEdit()
		return nil, err
	}
	return cmd, nil
}

func (m *Model) finishCommentEdit() {
	body := strings.TrimSpace(m.comments.body)
	if body == "" {
		if m.comments.editIndex >= 0 {
			m.deleteComment(m.comments.editIndex)
		}
		m.clearCommentEdit()
		m.cancelSelection()
		return
	}
	if m.dependencies.SaveComment == nil {
		m.err = fmt.Errorf("comment storage is unavailable")
		m.clearCommentEdit()
		return
	}
	var item comment.Comment
	if m.comments.editIndex >= 0 {
		item = m.comments.items[m.comments.editIndex]
		item.Body = body
	} else {
		item = comment.Comment{Anchor: m.comments.editAnchor, Body: body}
	}
	saved, err := m.dependencies.SaveComment(item, m.review.changes)
	if err != nil {
		m.err = err
		m.clearCommentEdit()
		return
	}
	if m.comments.editIndex >= 0 {
		m.comments.items[m.comments.editIndex] = saved
	} else {
		m.comments.items = append(m.comments.items, saved)
		m.comments.row = len(m.comments.items) - 1
	}
	m.comments.revision++
	m.clearCommentEdit()
	m.err = nil
	m.cancelSelection()
}

func (m *Model) deleteComment(index int) {
	if index < 0 || index >= len(m.comments.items) {
		return
	}
	if m.dependencies.DeleteComment == nil {
		m.err = fmt.Errorf("comment storage is unavailable")
		return
	}
	if err := m.dependencies.DeleteComment(m.comments.items[index], m.review.changes); err != nil {
		m.err = err
		return
	}
	m.comments.items = append(m.comments.items[:index], m.comments.items[index+1:]...)
	m.comments.row = min(m.comments.row, max(0, len(m.comments.items)-1))
	m.comments.revision++
	m.err = nil
}

func (m *Model) clearCommentEdit() {
	m.comments.body = ""
	m.comments.editIndex = -1
	m.comments.editAnchor = comment.Anchor{}
}

func (m *Model) cancelSelection() { m.review.selection = nil }

func (m Model) openCommentEditor() (tea.Cmd, error) {
	if m.dependencies.Editor == nil {
		return nil, fmt.Errorf("editor is unavailable")
	}
	return m.dependencies.Editor.EditComment(m.comments.body, m.comments.editAnchor)
}
