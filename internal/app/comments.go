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

func (m model) handleCommentsScreenKey(keyName string) (tea.Model, tea.Cmd) {
	m.err = nil
	switch keyName {
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
			m.deleteComment(m.commentIndex(selected))
		}
	default:
		return m, nil
	}
	return m, nil
}

func (m model) editSelectedComment() (tea.Model, tea.Cmd) {
	selected, ok := m.commentView.Selected()
	if !ok {
		return m, nil
	}
	m.edit.index = m.commentIndex(selected)
	if m.edit.index < 0 {
		return m, nil
	}
	m.edit.body = m.comments.items[m.edit.index].Body
	m.edit.anchor = m.comments.items[m.edit.index].Anchor
	cmd, err := m.openCommentEditor()
	if err != nil {
		m.err = err
		m.edit.resetDraft()
		return m, nil
	}
	return m, cmd
}

func (m *model) beginComment() (tea.Cmd, error) {
	file, lines, ok := m.diffView.Selected()
	if !ok {
		return nil, errors.New("select code lines before commenting")
	}
	anchor := commentAnchor(file, lines)
	m.edit.body, m.edit.index, m.edit.anchor = "", -1, anchor
	cmd, err := m.openCommentEditor()
	if err != nil {
		m.edit.resetDraft()
		return nil, err
	}
	return cmd, nil
}

func (m *model) persistCommentEdit(draft commentEdit) {
	body := strings.TrimSpace(draft.body)
	if body == "" {
		if draft.index >= 0 {
			m.deleteComment(draft.index)
		}
		m.edit.resetDraft()
		m.diffView.ClearSelection()
		return
	}
	if m.save == nil {
		m.err = errors.New("comment storage is unavailable")
		m.edit.resetDraft()
		return
	}
	var comment comments.Comment
	if draft.index >= 0 {
		comment = m.comments.items[draft.index]
		comment.Body = body
	} else {
		comment = comments.Comment{Anchor: draft.anchor, Body: body}
	}
	saved, err := m.save(comment, m.currentPatch)
	if err != nil {
		m.err = err
		m.edit.resetDraft()
		return
	}
	if draft.index >= 0 {
		m.comments.items[draft.index] = saved
	} else {
		m.comments.items = append(m.comments.items, saved)
		m.commentView.Update(m.comments.items)
		m.commentView.Move(len(m.comments.items))
	}
	m.comments.revision++
	m.commentView.Update(m.comments.items)
	m.edit.resetDraft()
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
	m.commentView.Update(m.comments.items)
	m.comments.revision++
	m.err = nil
}

func (draft *commentEdit) resetDraft() {
	*draft = commentEdit{index: -1}
}

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
	file, line, ok := m.diffView.Current()
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

func (m model) commentIndex(selected comments.Comment) int {
	for index, item := range m.comments.items {
		if selected.ID != "" && item.ID == selected.ID ||
			selected.ID == "" && item.Body == selected.Body &&
				item.Anchor.FilePath == selected.Anchor.FilePath {
			return index
		}
	}
	return -1
}

func commentAnchor(file patch.File, lines []patch.Line) comments.Anchor {
	path := file.NewPath
	if path == "" {
		path = file.OldPath
	}
	anchor := comments.Anchor{FilePath: path}
	var oldLines, newLines sourceLineRange
	for _, line := range lines {
		prefix := " "
		if line.Kind == patch.Addition {
			prefix = "+"
		}
		if line.Kind == patch.Deletion {
			prefix = "-"
		}
		anchor.QuotedLines = append(anchor.QuotedLines, prefix+line.Text)
		oldLines.includeLine(line.OldNumber)
		newLines.includeLine(line.NewNumber)
	}
	anchor.OldStart, anchor.OldEnd = oldLines.first, oldLines.last
	anchor.NewStart, anchor.NewEnd = newLines.first, newLines.last
	return anchor
}

// sourceLineRange tracks inclusive bounds, ignoring absent diff line numbers.
type sourceLineRange struct {
	first, last int
}

func (r *sourceLineRange) includeLine(number patch.LineNumber) {
	if number == 0 {
		return
	}
	if r.first == 0 || int(number) < r.first {
		r.first = int(number)
	}
	if int(number) > r.last {
		r.last = int(number)
	}
}
