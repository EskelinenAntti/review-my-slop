package app

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/eskelinenantti/review-my-slop/internal/comments"
)

var errCommentStorageUnavailable = errors.New("comment storage is unavailable")

type saveCommentFunc func(comments.Comment) (comments.Comment, error)
type deleteCommentFunc func(comments.Comment) error
type loadCommentsFunc func() ([]comments.Comment, error)

// commentState pairs stored comments with the revision used to reject stale
// loads.
type commentState struct {
	items    []comments.Comment
	revision uint64
}

type commentsLoadedMsg struct {
	comments []comments.Comment
	revision uint64
	err      error
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
	comment.Repository = m.currentPatch.Root
	saved, err := m.save(comment)
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
	comment := m.comments.items[index]
	comment.Repository = m.currentPatch.Root
	err := m.delete(comment)
	if err != nil {
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
