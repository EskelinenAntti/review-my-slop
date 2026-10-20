package app

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/eskelinenantti/review-my-slop/internal/comments"
	"github.com/eskelinenantti/review-my-slop/internal/editor"
)

var errCommentStorageUnavailable = errors.New("comment storage is unavailable")

func (m model) updateComments(name string) (tea.Model, tea.Cmd) {
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
		if index, ok := m.commentView.SelectedIndex(); ok {
			m.deleteComment(index)
		}
	default:
		return m, nil
	}
	return m, nil
}

func (m model) editSelectedComment() (tea.Model, tea.Cmd) {
	index, ok := m.commentView.SelectedIndex()
	if !ok {
		return m, nil
	}
	comment := m.comments.items[index]
	m.edit = commentEdit{
		body:   comment.Body,
		index:  index,
		anchor: comment.Anchor,
	}
	cmd, err := m.openCommentEditor()
	if err != nil {
		m.err = err
		return m, nil
	}
	return m, cmd
}

func (m *model) beginComment() (tea.Cmd, error) {
	file, lines, ok := m.diffView.Selected()
	if !ok {
		return nil, errors.New("select code lines before commenting")
	}
	anchor := comments.NewAnchor(file, lines)
	m.edit = commentEdit{index: -1, anchor: anchor}
	return m.openCommentEditor()
}

func (m *model) finishCommentEditor(msg commentEditorFinishedMsg) {
	if msg.err != nil {
		m.err = msg.err
		m.clearCommentEdit()
		return
	}
	m.edit.body = msg.body
	m.finishCommentEdit()
}

func (m *model) finishCommentEdit() {
	defer m.clearCommentEdit()
	body := strings.TrimSpace(m.edit.body)
	if body == "" {
		if m.edit.index >= 0 {
			m.deleteComment(m.edit.index)
		}
		m.diffView.ClearSelection()
		return
	}
	if m.save == nil {
		m.err = errCommentStorageUnavailable
		return
	}
	var comment comments.Comment
	if m.edit.index >= 0 {
		comment = m.comments.items[m.edit.index]
		comment.Body = body
	} else {
		comment = comments.Comment{Anchor: m.edit.anchor, Body: body}
	}
	saved, err := m.save(comment, m.currentPatch)
	if err != nil {
		m.err = err
		return
	}
	m.updateSavedComment(saved, m.edit.index)
	m.diffView.ClearSelection()
}

func (m *model) updateSavedComment(saved comments.Comment, index int) {
	if index >= 0 {
		m.comments.items[index] = saved
	} else {
		m.comments.items = append(m.comments.items, saved)
	}
	m.commentsChanged()
	if index < 0 {
		m.commentView.Move(len(m.comments.items))
	}
}

func (m *model) deleteComment(index int) {
	if index < 0 || index >= len(m.comments.items) {
		return
	}
	if m.delete == nil {
		m.err = errCommentStorageUnavailable
		return
	}
	if err := m.delete(m.comments.items[index], m.currentPatch); err != nil {
		m.err = err
		return
	}
	m.comments.items = slices.Delete(m.comments.items, index, index+1)
	m.commentsChanged()
}

func (m *model) commentsChanged() {
	m.commentView.Update(m.comments.items)
	m.comments.revision++
	m.err = nil
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

func (m *model) finishCommentsLoad(msg commentsLoadedMsg) {
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

func (m *model) clearCommentEdit() {
	m.edit = commentEdit{index: -1}
}

func (m *model) openCommentEditor() (tea.Cmd, error) {
	cmd, err := editor.EditComment(m.ctx, m.edit.body, m.edit.anchor, func(
		body string,
		err error,
	) tea.Msg {
		return commentEditorFinishedMsg{body: body, err: err}
	})
	if err != nil {
		m.clearCommentEdit()
	}
	return cmd, err
}
