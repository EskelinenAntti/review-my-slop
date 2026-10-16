package app

import (
	"errors"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/eskelinenantti/review-my-slop/internal/comments"
	"github.com/eskelinenantti/review-my-slop/internal/editor"
	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

func (m model) showCommentsForView(name string) (tea.Model, tea.Cmd) {
	m.err = nil
	switch name {
	case "esc", "C", "q":
		m.mode = modeBrowse
	case "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	case "j", "down":
		m.commentView.MoveCommentSelection(1)
	case "k", "up":
		m.commentView.MoveCommentSelection(-1)
	case "enter", "e":
		return m.editSelectedComment()
	case "D":
		if selected, ok := m.commentView.SelectedComment(); ok {
			m.deleteComment(m.indexOfComment(selected))
		}
	default:
		return m, nil
	}
	return m, nil
}

func (m model) editSelectedComment() (tea.Model, tea.Cmd) {
	selected, ok := m.commentView.SelectedComment()
	if !ok {
		return m, nil
	}
	m.edit.index = m.indexOfComment(selected)
	if m.edit.index < 0 {
		return m, nil
	}
	m.edit.body = m.comments.items[m.edit.index].Body
	m.edit.anchor = m.comments.items[m.edit.index].Anchor
	cmd, err := m.openCommentEditor()
	if err != nil {
		m.err = err
		m.cancelCommentEdit()
		return m, nil
	}
	return m, cmd
}

func (m *model) startNewComment() (tea.Cmd, error) {
	file, lines, ok := m.diffView.SelectedFileAndLines()
	if !ok {
		return nil, errors.New("select code lines before commenting")
	}
	anchor := anchorForComment(file, lines)
	m.edit.body, m.edit.index, m.edit.anchor = "", -1, anchor
	cmd, err := m.openCommentEditor()
	if err != nil {
		m.cancelCommentEdit()
		return nil, err
	}
	return cmd, nil
}

func (m *model) saveEditedComment() {
	body := strings.TrimSpace(m.edit.body)
	if body == "" {
		if m.edit.index >= 0 {
			m.deleteComment(m.edit.index)
		}
		m.cancelCommentEdit()
		m.diffView.ClearSelection()
		return
	}
	if m.save == nil {
		m.err = errors.New("comment storage is unavailable")
		m.cancelCommentEdit()
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
		m.cancelCommentEdit()
		return
	}
	if m.edit.index >= 0 {
		m.comments.items[m.edit.index] = saved
	} else {
		m.comments.items = append(m.comments.items, saved)
		m.commentView.ReplaceComments(m.comments.items)
		m.commentView.MoveCommentSelection(len(m.comments.items))
	}
	m.comments.revision++
	m.commentView.ReplaceComments(m.comments.items)
	m.cancelCommentEdit()
	m.err = nil
	m.diffView.ClearSelection()
}

func (m *model) deleteComment(index int) {
	if index < 0 || index >= len(m.comments.items) {
		return
	}
	if m.delete == nil {
		m.err = errors.New("comment storage is unavailable")
		return
	}
	if err := m.delete(m.comments.items[index], m.currentPatch); err != nil {
		m.err = err
		return
	}
	m.comments.items = append(
		m.comments.items[:index],
		m.comments.items[index+1:]...,
	)
	m.commentView.ReplaceComments(m.comments.items)
	m.comments.revision++
	m.err = nil
}

func (m *model) cancelCommentEdit() {
	m.edit = commentEdit{index: -1}
}

func (m model) openCurrentFileAtSelectedLine() (tea.Cmd, error) {
	path, number, err := m.selectedSourceFileAndLine()
	if err != nil {
		return nil, err
	}
	return editor.OpenSource(m.ctx, path, number, func(err error) tea.Msg {
		return sourceEditorFinishedMsg{err: err}
	})
}

func (m model) selectedSourceFileAndLine() (string, int, error) {
	file, line, ok := m.diffView.FocusedFileAndLine()
	if !ok {
		return "", 0, errors.New("select a code line to open in $EDITOR")
	}
	path, number := file.NewPath, line.NewNumber
	if path == "" || path == "/dev/null" {
		path = file.OldPath
	}
	if number == 0 {
		number = line.OldNumber
	}
	if path == "" || path == "/dev/null" || number < 1 {
		return "", 0, errors.New(
			"current line has no editable working-tree location",
		)
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(m.currentPatch.Root, filepath.FromSlash(path))
	}
	return path, int(number), nil
}

func (m model) openCommentEditor() (tea.Cmd, error) {
	return editor.EditComment(m.ctx, m.edit.body, m.edit.anchor, func(
		body string,
		err error,
	) tea.Msg {
		return commentEditorFinishedMsg{body: body, err: err}
	})
}

func (m model) indexOfComment(selected comments.Comment) int {
	for index, item := range m.comments.items {
		if selected.ID != "" && item.ID == selected.ID ||
			selected.ID == "" && item.Body == selected.Body &&
				item.Anchor.FilePath == selected.Anchor.FilePath {
			return index
		}
	}
	return -1
}

func anchorForComment(file patch.File, lines []patch.Line) comments.Anchor {
	path := file.NewPath
	if path == "" {
		path = file.OldPath
	}
	anchor := comments.Anchor{FilePath: path}
	for _, line := range lines {
		prefix := " "
		if line.Kind == patch.Addition {
			prefix = "+"
		}
		if line.Kind == patch.Deletion {
			prefix = "-"
		}
		anchor.QuotedLines = append(anchor.QuotedLines, prefix+line.Text)
		includeLineInRange(
			&anchor.OldStart,
			&anchor.OldEnd,
			int(line.OldNumber),
		)
		includeLineInRange(
			&anchor.NewStart,
			&anchor.NewEnd,
			int(line.NewNumber),
		)
	}
	return anchor
}
func includeLineInRange(start, end *int, value int) {
	if value == 0 {
		return
	}
	if *start == 0 || value < *start {
		*start = value
	}
	if value > *end {
		*end = value
	}
}
