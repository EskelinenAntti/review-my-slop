package app

import (
	"errors"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/eskelinenantti/review-my-slop/internal/comments"
	"github.com/eskelinenantti/review-my-slop/internal/editor"
)

func (m model) handleCommentModeKey(name string) (tea.Model, tea.Cmd) {
	m.err = nil
	switch name {
	case "esc", "C", "q":
		m.mode = modeBrowse
	case "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	case "j", "down":
		m.commentView.Move(1)
	case "k", "up":
		m.commentView.Move(-1)
	case "enter", "e":
		return m.editSelectedComment()
	case "D":
		if selected, ok := m.commentView.Selected(); ok {
			m.deleteComment(m.findCommentIndex(selected))
		}
	default:
		return m, nil
	}
	return m, nil
}

func (m *model) startNewComment() (tea.Cmd, error) {
	file, lines, ok := m.patchView.Selected()
	if !ok {
		return nil, errors.New("select code lines before commenting")
	}
	anchor := commentAnchor(file, lines)
	m.edit.body, m.edit.index, m.edit.anchor = "", -1, anchor
	cmd, err := m.launchCommentEditor()
	if err != nil {
		m.clearCommentDraft()
		return nil, err
	}
	return cmd, nil
}

func (m model) editSelectedComment() (tea.Model, tea.Cmd) {
	selected, ok := m.commentView.Selected()
	if !ok {
		return m, nil
	}
	m.edit.index = m.findCommentIndex(selected)
	if m.edit.index < 0 {
		return m, nil
	}
	m.edit.body = m.comments.items[m.edit.index].Body
	m.edit.anchor = m.comments.items[m.edit.index].Anchor
	cmd, err := m.launchCommentEditor()
	if err != nil {
		m.err = err
		m.clearCommentDraft()
		return m, nil
	}
	return m, cmd
}

func (m model) launchCommentEditor() (tea.Cmd, error) {
	return editor.EditComment(m.ctx, m.edit.body, m.edit.anchor, func(
		body string,
		err error,
	) tea.Msg {
		return commentEditorFinishedMsg{body: body, err: err}
	})
}

func (m *model) completeCommentEditor(msg commentEditorFinishedMsg) {
	if msg.err != nil {
		m.err = msg.err
		m.clearCommentDraft()
		return
	}
	m.edit.body = msg.body
	m.saveCommentDraft()
}

func (m *model) saveCommentDraft() {
	body := strings.TrimSpace(m.edit.body)
	if body == "" {
		if m.edit.index >= 0 {
			m.deleteComment(m.edit.index)
		}
		m.clearCommentDraft()
		m.patchView.ClearSelection()
		return
	}
	if m.saveCommentToStore == nil {
		m.err = errors.New("comment storage is unavailable")
		m.clearCommentDraft()
		return
	}
	var comment comments.Comment
	if m.edit.index >= 0 {
		comment = m.comments.items[m.edit.index]
		comment.Body = body
	} else {
		comment = comments.Comment{Anchor: m.edit.anchor, Body: body}
	}
	saved, err := m.saveCommentToStore(comment, m.currentPatch)
	if err != nil {
		m.err = err
		m.clearCommentDraft()
		return
	}
	if m.edit.index >= 0 {
		m.comments.items[m.edit.index] = saved
	} else {
		m.comments.items = append(m.comments.items, saved)
		m.commentView.Update(m.comments.items)
		m.commentView.Move(len(m.comments.items))
	}
	m.comments.revision++
	m.commentView.Update(m.comments.items)
	m.clearCommentDraft()
	m.err = nil
	m.patchView.ClearSelection()
}

func (m *model) deleteComment(index int) {
	if index < 0 || index >= len(m.comments.items) {
		return
	}
	if m.deleteCommentFromStore == nil {
		m.err = errors.New("comment storage is unavailable")
		return
	}
	err := m.deleteCommentFromStore(
		m.comments.items[index], m.currentPatch,
	)
	if err != nil {
		m.err = err
		return
	}
	m.comments.items = append(
		m.comments.items[:index],
		m.comments.items[index+1:]...,
	)
	m.commentView.Update(m.comments.items)
	m.comments.revision++
	m.err = nil
}

func (m model) findCommentIndex(selected comments.Comment) int {
	for index, item := range m.comments.items {
		if selected.ID != "" && item.ID == selected.ID ||
			selected.ID == "" && item.Body == selected.Body &&
				item.Anchor.FilePath == selected.Anchor.FilePath {
			return index
		}
	}
	return -1
}

func (m *model) clearCommentDraft() {
	m.edit = commentEdit{index: -1}
}
