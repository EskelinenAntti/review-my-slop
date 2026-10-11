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
	default:
		return m.moveComments(name)
	}
	return m, nil
}

func (m model) moveComments(name string) (tea.Model, tea.Cmd) {
	switch name {
	case "j", "down":
		m.commentView.Move(1)
	case "k", "up":
		m.commentView.Move(-1)
	default:
		return m.commentAction(name)
	}
	return m, nil
}

func (m model) commentAction(name string) (tea.Model, tea.Cmd) {
	switch name {
	case "enter", "e":
		return m.editSelectedComment()
	case "D":
		if selected, ok := m.commentView.Selected(); ok {
			m.deleteComment(m.commentIndex(selected))
		}
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
		m.clearCommentEdit()
		return m, nil
	}
	return m, cmd
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
		m.finishEmptyComment()
		return
	}
	if m.save == nil {
		m.err = fmt.Errorf("comment storage is unavailable")
		m.clearCommentEdit()
		return
	}
	comment := m.editedComment(body)
	saved, err := m.save(comment, m.currentPatch)
	if err != nil {
		m.err = err
		m.clearCommentEdit()
		return
	}
	m.applySavedComment(saved)
	m.comments.revision++
	m.commentView.Update(m.comments.items)
	m.clearCommentEdit()
	m.err = nil
	m.diffView.ClearSelection()
}

func (m *model) deleteComment(index int) {
	if !m.validCommentIndex(index) {
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
	return m.resolveSourceLocation(file, line)
}

func (m model) resolveSourceLocation(file patch.File, line patch.Line) (string, int, error) {
	path, number := file.NewPath, line.NewNumber
	if !editablePath(path) {
		path = file.OldPath
	}
	if number == 0 {
		number = line.OldNumber
	}
	return m.validateSourceLocation(path, int(number))
}

func (m model) validateSourceLocation(path string, number int) (string, int, error) {
	if !editablePath(path) || number < 1 {
		return "", 0, fmt.Errorf("current line has no editable working-tree location")
	}
	return workingTreePath(m.currentPatch.Root, path), int(number), nil
}

func (m model) openCommentEditor() (tea.Cmd, error) {
	return editor.EditComment(m.ctx, m.edit.body, m.edit.anchor, func(body string, err error) tea.Msg {
		return commentEditorFinishedMsg{body: body, err: err}
	})
}

func (m model) commentIndex(selected comments.Comment) int {
	for index, item := range m.comments.items {
		if sameComment(selected, item) {
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
		prefix := " "
		if line.Kind == patch.Addition {
			prefix = "+"
		}
		if line.Kind == patch.Deletion {
			prefix = "-"
		}
		anchor.QuotedLines = append(anchor.QuotedLines, prefix+line.Text)
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

func (m model) editedComment(body string) comments.Comment {
	if m.edit.index >= 0 {
		comment := m.comments.items[m.edit.index]
		comment.Body = body
		return comment
	}
	return comments.Comment{Anchor: m.edit.anchor, Body: body}
}

func (m *model) applySavedComment(saved comments.Comment) {
	if m.edit.index >= 0 {
		m.comments.items[m.edit.index] = saved
	} else {
		m.comments.items = append(m.comments.items, saved)
		m.commentView.Update(m.comments.items)
		m.commentView.Move(len(m.comments.items))
	}
}

func editablePath(path string) bool { return path != "" && path != "/dev/null" }
func workingTreePath(root, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(root, filepath.FromSlash(path))
}

func sameComment(selected, item comments.Comment) bool {
	if selected.ID != "" {
		return item.ID == selected.ID
	}
	return item.Body == selected.Body && item.Anchor.FilePath == selected.Anchor.FilePath
}

func (m *model) finishEmptyComment() {
	if m.edit.index >= 0 {
		m.deleteComment(m.edit.index)
	}
	m.clearCommentEdit()
	m.diffView.ClearSelection()
}

func (m model) validCommentIndex(index int) bool { return index >= 0 && index < len(m.comments.items) }
