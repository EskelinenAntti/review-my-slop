package app

import (
	"errors"
	"path/filepath"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/eskelinenantti/review-my-slop/internal/comments"
	"github.com/eskelinenantti/review-my-slop/internal/editor"
	"github.com/eskelinenantti/review-my-slop/internal/patch"
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
	return m.startEditor(m.openCommentEditor)
}

func (m *model) beginComment() (tea.Cmd, error) {
	file, lines, ok := m.diffView.Selected()
	if !ok {
		return nil, errors.New("select code lines before commenting")
	}
	anchor := commentAnchor(file, lines)
	m.edit = commentEdit{index: -1, anchor: anchor}
	return m.openCommentEditor()
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

func (m *model) clearCommentEdit() {
	m.edit = commentEdit{index: -1}
}

func (m *model) openCurrentLine() (tea.Cmd, error) {
	path, number, err := m.sourceLocation()
	if err != nil {
		return nil, err
	}
	return editor.OpenSource(m.ctx, path, number, func(err error) tea.Msg {
		return sourceEditorFinishedMsg{err: err}
	})
}

func (m model) sourceLocation() (string, int, error) {
	file, line, ok := m.diffView.Current()
	if !ok {
		return "", 0, errors.New("select a code line to open in $EDITOR")
	}
	path, number := file.NewPath, line.NewNumber
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

func commentAnchor(file patch.File, lines []patch.Line) comments.Anchor {
	anchor := comments.Anchor{FilePath: file.PreferredPath()}
	for _, line := range lines {
		anchor.QuotedLines = append(
			anchor.QuotedLines,
			line.Kind.DiffPrefix()+line.Text,
		)
		accumulateRange(&anchor.OldStart, &anchor.OldEnd, int(line.OldNumber))
		accumulateRange(&anchor.NewStart, &anchor.NewEnd, int(line.NewNumber))
	}
	return anchor
}

func accumulateRange(start, end *int, value int) {
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
