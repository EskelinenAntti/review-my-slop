package ui

import (
	"fmt"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/eskelinenantti/review-my-slop/internal/comments"
	"github.com/eskelinenantti/review-my-slop/internal/editor"
)

func (m model) updateComments(name string) (tea.Model, tea.Cmd) {
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

func (m *model) beginComment() (tea.Cmd, error) {
	selection := m.review.selection
	if selection == nil {
		current := m.review.view.beginSelection(m.review.cursor)
		selection = &current
	}
	anchor, err := m.review.view.anchor(*selection)
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

func (m *model) finishCommentEdit() {
	body := strings.TrimSpace(m.comments.body)
	if body == "" {
		if m.comments.editIndex >= 0 {
			m.deleteComment(m.comments.editIndex)
		}
		m.clearCommentEdit()
		m.cancelSelection()
		return
	}
	if m.save == nil {
		m.err = fmt.Errorf("comment storage is unavailable")
		m.clearCommentEdit()
		return
	}
	var comment comments.Comment
	if m.comments.editIndex >= 0 {
		comment = m.comments.items[m.comments.editIndex]
		comment.Body = body
	} else {
		comment = comments.Comment{Anchor: m.comments.editAnchor, Body: body}
	}
	saved, err := m.save(comment, m.review.patch)
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

func (m *model) deleteComment(index int) {
	if index < 0 || index >= len(m.comments.items) {
		return
	}
	if m.delete == nil {
		m.err = fmt.Errorf("comment storage is unavailable")
		return
	}
	if err := m.delete(m.comments.items[index], m.review.patch); err != nil {
		m.err = err
		return
	}
	m.comments.items = append(m.comments.items[:index], m.comments.items[index+1:]...)
	m.comments.row = min(m.comments.row, max(0, len(m.comments.items)-1))
	m.comments.revision++
	m.err = nil
}

func (m *model) clearCommentEdit() {
	m.comments.body = ""
	m.comments.editIndex = -1
	m.comments.editAnchor = comments.Anchor{}
}

func (m *model) cancelSelection() { m.review.selection = nil }

func (m model) openCurrentLine() (tea.Cmd, error) {
	path, number, err := m.sourceLocation()
	if err != nil {
		return nil, err
	}
	return editor.OpenSource(m.ctx, path, number, func(err error) tea.Msg {
		return sourceEditorFinishedMsg{err: err}
	})
}

func (m model) sourceLocation() (string, int, error) {
	file, fileOK := m.review.view.file(m.review.cursor)
	line, lineOK := m.review.view.line(m.review.cursor)
	if !fileOK || !lineOK {
		return "", 0, fmt.Errorf("select a code line to open in $EDITOR")
	}
	path, number := file.NewPath, line.NewNumber
	if path == "" || path == "/dev/null" {
		path = file.OldPath
	}
	if number == 0 {
		number = line.OldNumber
	}
	if path == "" || path == "/dev/null" || number < 1 {
		return "", 0, fmt.Errorf("current line has no editable working-tree location")
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(m.review.patch.Root, filepath.FromSlash(path))
	}
	return path, int(number), nil
}

func (m model) openCommentEditor() (tea.Cmd, error) {
	return editor.EditComment(m.ctx, m.comments.body, m.comments.editAnchor, func(body string, err error) tea.Msg {
		return commentEditorFinishedMsg{body: body, err: err}
	})
}
