package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/eskelinenantti/review-my-slop/internal/editor"
	"github.com/eskelinenantti/review-my-slop/internal/review"
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
		if m.commentRow < len(m.comments)-1 {
			m.commentRow++
		}
	case "k", "up":
		if m.commentRow > 0 {
			m.commentRow--
		}
	case "enter", "e":
		if len(m.comments) > 0 {
			m.editIndex = m.commentRow
			m.commentBody = m.comments[m.editIndex].Body
			m.editAnchor = m.comments[m.editIndex].Anchor
			cmd, err := m.openCommentEditor()
			if err != nil {
				m.err = err
				m.clearCommentEdit()
				return m, nil
			}
			return m, cmd
		}
	case "D":
		if len(m.comments) > 0 {
			m.deleteComment(m.commentRow)
		}
	}
	return m, nil
}

func (m *Model) beginComment() (tea.Cmd, error) {
	selection := m.selection
	if selection == nil {
		current := m.view.BeginSelection(m.cursor)
		selection = &current
	}
	anchor, err := m.view.Anchor(*selection)
	if err != nil {
		return nil, err
	}
	m.commentBody, m.editIndex, m.editAnchor = "", -1, anchor
	cmd, err := m.openCommentEditor()
	if err != nil {
		m.clearCommentEdit()
		return nil, err
	}
	return cmd, nil
}

func (m *Model) finishCommentEdit() {
	body := strings.TrimSpace(m.commentBody)
	if body == "" {
		if m.editIndex >= 0 {
			m.deleteComment(m.editIndex)
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
	var comment review.Comment
	if m.editIndex >= 0 {
		comment = m.comments[m.editIndex]
		comment.Body = body
	} else {
		comment = review.Comment{Anchor: m.editAnchor, Body: body}
	}
	saved, err := m.save(comment, m.patch)
	if err != nil {
		m.err = err
		m.clearCommentEdit()
		return
	}
	if m.editIndex >= 0 {
		m.comments[m.editIndex] = saved
	} else {
		m.comments = append(m.comments, saved)
		m.commentRow = len(m.comments) - 1
	}
	m.clearCommentEdit()
	m.err = nil
	m.cancelSelection()
}

func (m *Model) deleteComment(index int) {
	if index < 0 || index >= len(m.comments) {
		return
	}
	if m.delete == nil {
		m.err = fmt.Errorf("comment storage is unavailable")
		return
	}
	if err := m.delete(m.comments[index], m.patch); err != nil {
		m.err = err
		return
	}
	m.comments = append(m.comments[:index], m.comments[index+1:]...)
	m.commentRow = min(m.commentRow, max(0, len(m.comments)-1))
	m.err = nil
}

func (m *Model) clearCommentEdit() {
	m.commentBody = ""
	m.editIndex = -1
	m.editAnchor = review.Anchor{}
}

func (m *Model) cancelSelection() { m.selection = nil }

func (m Model) openCurrentLine() (tea.Cmd, error) {
	editorCommand := strings.TrimSpace(os.Getenv("EDITOR"))
	if editorCommand == "" {
		return nil, fmt.Errorf("$EDITOR is not set")
	}
	file, fileOK := m.view.File(m.cursor)
	line, lineOK := m.view.Line(m.cursor)
	if !fileOK || !lineOK {
		return nil, fmt.Errorf("select a code line to open in $EDITOR")
	}
	path, number := file.NewPath, line.NewNumber
	if path == "" || path == "/dev/null" {
		path = file.OldPath
	}
	if number == 0 {
		number = line.OldNumber
	}
	if path == "" || path == "/dev/null" || number < 1 {
		return nil, fmt.Errorf("current line has no editable working-tree location")
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(m.patch.Repository, filepath.FromSlash(path))
	}
	return tea.ExecProcess(editor.SourceCommand(editorCommand, path, int(number)), func(err error) tea.Msg {
		return sourceEditorFinishedMsg{err: err}
	}), nil
}

func (m Model) openCommentEditor() (tea.Cmd, error) {
	editorCommand := strings.TrimSpace(os.Getenv("EDITOR"))
	if editorCommand == "" {
		return nil, fmt.Errorf("$EDITOR is not set")
	}
	path, err := editor.CreateCommentFile(m.commentBody, m.editAnchor)
	if err != nil {
		return nil, err
	}
	return tea.ExecProcess(editor.CommentCommand(editorCommand, path), func(editorErr error) tea.Msg {
		body, err := editor.ReadCommentFile(path, m.editAnchor, editorErr)
		return commentEditorFinishedMsg{body: body, err: err}
	}), nil
}
