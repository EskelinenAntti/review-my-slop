package ui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/eskelinenantti/review-my-slop/internal/comments"
)

func (m Model) updateComments(name string) (tea.Model, tea.Cmd) {
	state := &m.comments
	items := state.items
	m.err = nil
	switch name {
	case "esc", "C", "q":
		m.mode = modeBrowse
	case "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	case "j", "down":
		if state.row < len(items)-1 {
			state.row++
		}
	case "k", "up":
		if state.row > 0 {
			state.row--
		}
	case "enter", "e":
		if len(items) > 0 {
			state.editIndex = state.row
			state.body = items[state.editIndex].Body
			state.editAnchor = items[state.editIndex].Anchor
			cmd, err := m.openCommentEditor()
			if err != nil {
				m.err = err
				m.clearCommentEdit()
				return m, nil
			}
			return m, cmd
		}
	case "D":
		if len(items) > 0 {
			m.deleteComment(state.row)
		}
	}
	return m, nil
}

func (m *Model) beginComment() (tea.Cmd, error) {
	review, state := &m.review, &m.comments
	selection := review.selection
	if selection == nil {
		current := Selection{review.cursor, review.cursor}
		selection = &current
	}
	anchor, err := review.view.Anchor(*selection)
	if err != nil {
		return nil, err
	}
	state.body, state.editIndex, state.editAnchor = "", -1, anchor
	cmd, err := m.openCommentEditor()
	if err != nil {
		m.clearCommentEdit()
		return nil, err
	}
	return cmd, nil
}

func (m *Model) finishCommentEdit() {
	state := &m.comments
	body := strings.TrimSpace(state.body)
	index := state.editIndex
	editing := index >= 0
	if body == "" {
		if editing {
			m.deleteComment(index)
		}
		m.clearCommentEdit()
		m.review.selection = nil
		return
	}
	if m.save == nil {
		m.err = fmt.Errorf("comment storage is unavailable")
		m.clearCommentEdit()
		return
	}
	var comment comments.Comment
	if editing {
		comment = state.items[index]
		comment.Body = body
	} else {
		comment = comments.Comment{Anchor: state.editAnchor, Body: body}
	}
	saved, err := m.save(comment, m.review.patch)
	if err != nil {
		m.err = err
		m.clearCommentEdit()
		return
	}
	if editing {
		state.items[index] = saved
	} else {
		state.items = append(state.items, saved)
		state.row = len(state.items) - 1
	}
	state.revision++
	m.clearCommentEdit()
	m.err = nil
	m.review.selection = nil
}

func (m *Model) deleteComment(index int) {
	state := &m.comments
	if index < 0 || index >= len(state.items) {
		return
	}
	if m.delete == nil {
		m.err = fmt.Errorf("comment storage is unavailable")
		return
	}
	if err := m.delete(state.items[index], m.review.patch); err != nil {
		m.err = err
		return
	}
	state.items = slices.Delete(state.items, index, index+1)
	state.row = min(state.row, max(0, len(state.items)-1))
	state.revision++
	m.err = nil
}

func (m *Model) clearCommentEdit() {
	m.comments.body, m.comments.editIndex, m.comments.editAnchor = "", -1, comments.Anchor{}
}

func (m Model) openCurrentLine() (tea.Cmd, error) {
	review := &m.review
	editorCommand := strings.TrimSpace(os.Getenv("EDITOR"))
	if editorCommand == "" {
		return nil, fmt.Errorf("$EDITOR is not set")
	}
	file, fileOK := review.view.File(review.cursor)
	line, lineOK := review.view.Line(review.cursor)
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
		path = filepath.Join(review.patch.Repository, filepath.FromSlash(path))
	}
	return tea.ExecProcess(exec.Command("sh", "-c", editorCommand+" +"+strconv.Itoa(int(number))+" '"+strings.ReplaceAll(path, "'", "'\"'\"'")+"'"), func(err error) tea.Msg {
		return sourceEditorFinishedMsg{err}
	}), nil
}

func (m Model) openCommentEditor() (tea.Cmd, error) {
	state := &m.comments
	editorCommand := strings.TrimSpace(os.Getenv("EDITOR"))
	if editorCommand == "" {
		return nil, fmt.Errorf("$EDITOR is not set")
	}
	path, err := CreateCommentFile(state.body, state.editAnchor)
	if err != nil {
		return nil, err
	}
	return tea.ExecProcess(exec.Command("sh", "-c", editorCommand+" '"+strings.ReplaceAll(path, "'", "'\"'\"'")+"'"), func(editorErr error) tea.Msg {
		body, err := ReadCommentFile(path, state.editAnchor, editorErr)
		return commentEditorFinishedMsg{body, err}
	}), nil
}
