package app

import (
	"fmt"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/eskelinenantti/review-my-slop/internal/comments"
	"github.com/eskelinenantti/review-my-slop/internal/editor"
	"github.com/eskelinenantti/review-my-slop/internal/patch"
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
		m.commentView.Move(1)
	case "k", "up":
		m.commentView.Move(-1)
	case "enter", "e":
		return m.editSelectedComment()
	case "D":
		if selected, ok := m.commentView.Selected(); ok {
			m.deleteComment(m.commentIndex(selected))
		}
	}
	return m, nil
}

func (m *model) beginComment() (tea.Cmd, error) {
	file, lines, ok := m.diffView.Selected()
	if !ok {
		return nil, fmt.Errorf("select code lines before commenting")
	}
	anchor := commentAnchor(file, lines)
	m.edit.body, m.edit.index, m.edit.anchor = "", -1, anchor
	cmd, err := m.openCommentEditor()
	if err != nil {
		m.clearCommentEdit()
		return nil, err
	}
	return cmd, nil
}

func (m *model) finishCommentEdit() {
	body := strings.TrimSpace(m.edit.body)
	if body == "" {
		m.cancelEmptyComment()
		return
	}
	saved, err := m.saveEditedComment(body)
	if err != nil {
		m.err = err
		m.clearCommentEdit()
		return
	}
	m.acceptSavedComment(saved)
}

func (m *model) cancelEmptyComment() {
	if m.edit.index >= 0 {
		m.deleteComment(m.edit.index)
	}
	m.clearCommentEdit()
	m.diffView.ClearSelection()
}

func (m model) saveEditedComment(body string) (comments.Comment, error) {
	if m.save == nil {
		return comments.Comment{}, fmt.Errorf("comment storage is unavailable")
	}
	comment := comments.Comment{Anchor: m.edit.anchor, Body: body}
	if m.edit.index >= 0 {
		comment = m.comments.items[m.edit.index]
		comment.Body = body
	}
	return m.save(comment, m.currentPatch)
}

func (m *model) acceptSavedComment(saved comments.Comment) {
	if m.edit.index >= 0 {
		m.comments.items[m.edit.index] = saved
	} else {
		m.comments.items = append(m.comments.items, saved)
		m.commentView.Update(m.comments.items)
		m.commentView.Move(len(m.comments.items))
	}
	m.comments.revision++
	m.commentView.Update(m.comments.items)
	m.clearCommentEdit()
	m.err = nil
	m.diffView.ClearSelection()
}

func (m *model) deleteComment(index int) {
	if index < 0 || index >= len(m.comments.items) {
		return
	}
	if m.delete == nil {
		m.err = fmt.Errorf("comment storage is unavailable")
		return
	}
	if err := m.delete(m.comments.items[index], m.currentPatch); err != nil {
		m.err = err
		return
	}
	m.comments.items = append(m.comments.items[:index], m.comments.items[index+1:]...)
	m.commentView.Update(m.comments.items)
	m.comments.revision++
	m.err = nil
}

func (m *model) clearCommentEdit() {
	m.edit = commentEdit{index: -1}
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
		return "", 0, fmt.Errorf("select a code line to open in $EDITOR")
	}
	path, number := editableLocation(file, line)
	if !editablePath(path) || number < 1 {
		return "", 0, fmt.Errorf("current line has no editable working-tree location")
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(m.currentPatch.Root, filepath.FromSlash(path))
	}
	return path, int(number), nil
}

func (m model) openCommentEditor() (tea.Cmd, error) {
	return editor.EditComment(m.ctx, m.edit.body, m.edit.anchor, func(body string, err error) tea.Msg {
		return commentEditorFinishedMsg{body: body, err: err}
	})
}

func (m model) commentIndex(selected comments.Comment) int {
	for index, item := range m.comments.items {
		if sameComment(item, selected) {
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
	for _, line := range lines {
		anchor.QuotedLines = append(anchor.QuotedLines, quotedLine(line))
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
		m.clearCommentEdit()
	}
	return m, cmd
}

func editablePath(path string) bool { return path != "" && path != "/dev/null" }

func editableLocation(file patch.File, line patch.Line) (string, patch.LineNumber) {
	path, number := file.NewPath, line.NewNumber
	if !editablePath(path) {
		path = file.OldPath
	}
	if number == 0 {
		number = line.OldNumber
	}
	return path, number
}

func sameComment(item, selected comments.Comment) bool {
	if selected.ID != "" {
		return item.ID == selected.ID
	}
	return item.Body == selected.Body && item.Anchor.FilePath == selected.Anchor.FilePath
}

func quotedLine(line patch.Line) string {
	switch line.Kind {
	case patch.Addition:
		return "+" + line.Text
	case patch.Deletion:
		return "-" + line.Text
	default:
		return " " + line.Text
	}
}
