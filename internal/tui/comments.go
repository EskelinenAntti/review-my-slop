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

type commentState struct {
	items      []review.Comment
	row        int
	body       string
	editIndex  int
	editAnchor review.Anchor
	revision   uint64
}

func (s *commentState) beginEdit(index int) bool {
	if index < 0 || index >= len(s.items) {
		return false
	}
	s.editIndex = index
	s.body = s.items[index].Body
	s.editAnchor = s.items[index].Anchor
	return true
}

func (s *commentState) beginNew(anchor review.Anchor) {
	s.body = ""
	s.editIndex = -1
	s.editAnchor = anchor
}

func (s commentState) pendingComment(body string) review.Comment {
	if s.editIndex >= 0 {
		comment := s.items[s.editIndex]
		comment.Body = body
		return comment
	}
	return review.Comment{Anchor: s.editAnchor, Body: body}
}

func (s *commentState) store(comment review.Comment) {
	if s.editIndex >= 0 {
		s.items[s.editIndex] = comment
	} else {
		s.items = append(s.items, comment)
		s.row = len(s.items) - 1
	}
	s.revision++
}

func (s *commentState) remove(index int) {
	s.items = append(s.items[:index], s.items[index+1:]...)
	s.row = min(s.row, max(0, len(s.items)-1))
	s.revision++
}

func (s *commentState) setItems(items []review.Comment) {
	s.items = items
	s.row = min(s.row, max(0, len(s.items)-1))
}

func (s *commentState) clearEdit() {
	s.body = ""
	s.editIndex = -1
	s.editAnchor = review.Anchor{}
}

func (m Model) updateComments(name string) (tea.Model, tea.Cmd) {
	m.err = nil
	switch name {
	case "esc", "C", "q":
		m.mode = modeBrowse
	case "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	case "j", "down":
		m.moveCommentRow(1)
	case "k", "up":
		m.moveCommentRow(-1)
	case "enter", "e":
		cmd := m.editCurrentComment()
		return m, cmd
	case "D":
		m.removeComment(m.comments.row)
	}
	return m, nil
}

func (m *Model) moveCommentRow(delta int) {
	m.comments.row = min(max(0, m.comments.row+delta), max(0, len(m.comments.items)-1))
}

func (m *Model) editCurrentComment() tea.Cmd {
	if !m.comments.beginEdit(m.comments.row) {
		return nil
	}
	cmd, err := m.openCommentEditor()
	if err != nil {
		m.err = err
		m.comments.clearEdit()
		return nil
	}
	return cmd
}

func (m *Model) beginComment() (tea.Cmd, error) {
	selection := m.review.selection
	if selection == nil {
		currentSelection := m.review.view.BeginSelection(m.review.cursor)
		selection = &currentSelection
	}
	file, fileFound := m.review.view.File(selection.First)
	lines := m.review.view.Lines(*selection)
	if !fileFound || len(lines) == 0 {
		return nil, fmt.Errorf("select code lines before commenting")
	}
	anchor := review.NewAnchor(file.Path(), lines)
	m.comments.beginNew(anchor)
	cmd, err := m.openCommentEditor()
	if err != nil {
		m.comments.clearEdit()
		return nil, err
	}
	return cmd, nil
}

func (m *Model) finishCommentEdit() {
	body := strings.TrimSpace(m.comments.body)
	if body == "" {
		if m.comments.editIndex >= 0 {
			m.removeComment(m.comments.editIndex)
		}
		m.comments.clearEdit()
		m.cancelSelection()
		return
	}
	if m.saveComment == nil {
		m.err = fmt.Errorf("comment storage is unavailable")
		m.comments.clearEdit()
		return
	}
	saved, err := m.saveComment(m.comments.pendingComment(body), m.review.patch)
	if err != nil {
		m.err = err
		m.comments.clearEdit()
		return
	}
	m.comments.store(saved)
	m.comments.clearEdit()
	m.err = nil
	m.cancelSelection()
}

func (m *Model) removeComment(index int) {
	if index < 0 || index >= len(m.comments.items) {
		return
	}
	if m.deleteComment == nil {
		m.err = fmt.Errorf("comment storage is unavailable")
		return
	}
	comment := m.comments.items[index]
	if err := m.deleteComment(comment, m.review.patch); err != nil {
		m.err = err
		return
	}
	m.comments.remove(index)
	m.err = nil
}

func (m Model) openCurrentLine() (tea.Cmd, error) {
	editorCommand, err := configuredEditor()
	if err != nil {
		return nil, err
	}
	file, fileFound := m.review.view.File(m.review.cursor)
	line, lineFound := m.review.view.Line(m.review.cursor)
	if !fileFound || !lineFound {
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
		path = filepath.Join(m.review.patch.Repository, filepath.FromSlash(path))
	}
	return tea.ExecProcess(editor.SourceCommand(editorCommand, path, int(number)), func(err error) tea.Msg {
		return sourceEditorFinishedMsg{err: err}
	}), nil
}

func (m Model) openCommentEditor() (tea.Cmd, error) {
	editorCommand, err := configuredEditor()
	if err != nil {
		return nil, err
	}
	path, err := editor.CreateCommentFile(m.comments.body, m.comments.editAnchor)
	if err != nil {
		return nil, err
	}
	return tea.ExecProcess(editor.CommentCommand(editorCommand, path), func(editorErr error) tea.Msg {
		if editorErr != nil {
			_ = os.Remove(path)
			return commentEditorFinishedMsg{err: fmt.Errorf("editor: %w", editorErr)}
		}
		body, err := editor.ReadCommentFile(path, m.comments.editAnchor)
		return commentEditorFinishedMsg{body: body, err: err}
	}), nil
}

func configuredEditor() (string, error) {
	command := strings.TrimSpace(os.Getenv("EDITOR"))
	if command == "" {
		return "", fmt.Errorf("$EDITOR is not set")
	}
	return command, nil
}
