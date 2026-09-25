package ui

import (
	"errors"
	"os"
	"path/filepath"

	tea "charm.land/bubbletea/v2"
)

var (
	newError                     = errors.New
	errCommentStorageUnavailable = newError("comment storage is unavailable")
	errEditorUnset               = newError("$EDITOR is not set")
)

func (m Model) updateComments(name string) (teaModel, teaCmd) {
	comment := &m.comments
	items, row := comment.items, comment.row
	hasComments := len(items) > 0
	m.err = nil
	switch name {
	case "esc", "C", "q":
		m.mode = modeBrowse
	case "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	case "j", "down":
		if row < len(items)-1 {
			row++
		}
	case "k", "up":
		if row > 0 {
			row--
		}
	case "enter", "e":
		if hasComments {
			index := row
			comment.editIndex = index
			comment.body = items[index].Body
			comment.editAnchor = items[index].Anchor
			cmd, err := m.openCommentEditor()
			if err != nil {
				m.err = err
				m.clearCommentEdit()
				return m, nil
			}
			return m, cmd
		}
	case "D":
		if hasComments {
			m.deleteComment(row)
		}
		return m, nil
	}
	comment.row = row
	return m, nil
}

func (m *Model) beginComment() (teaCmd, error) {
	review, comment := &m.review, &m.comments
	view := review.view
	selection := review.selection
	if selection == nil {
		current := view.BeginSelection(review.cursor)
		selection = &current
	}
	anchor, err := view.Anchor(*selection)
	if err != nil {
		return nil, err
	}
	comment.body, comment.editIndex, comment.editAnchor = "", -1, anchor
	cmd, err := m.openCommentEditor()
	if err != nil {
		m.clearCommentEdit()
		return nil, err
	}
	return cmd, nil
}

func (m *Model) finishCommentEdit() {
	defer m.clearCommentEdit()
	state := &m.comments
	save, cancelSelection := m.save, m.cancelSelection
	items, index := state.items, state.editIndex
	body := trimSpace(state.body)
	editing := index >= 0
	if body == "" {
		if editing {
			m.deleteComment(state.editIndex)
		}
		cancelSelection()
		return
	}
	if save == nil {
		m.err = errCommentStorageUnavailable
		return
	}
	var comment reviewComment
	if editing {
		comment = items[index]
		comment.Body = body
	} else {
		comment = reviewComment{Anchor: state.editAnchor, Body: body}
	}
	saved, err := save(comment, m.review.patch)
	if err != nil {
		m.err = err
		return
	}
	if editing {
		items[index] = saved
	} else {
		items = append(items, saved)
		state.row = len(items) - 1
	}
	state.items = items
	state.revision++
	m.err = nil
	cancelSelection()
}

func (m *Model) deleteComment(index int) {
	comment := &m.comments
	delete := m.delete
	items := comment.items
	if index < 0 || index >= len(items) {
		return
	}
	if delete == nil {
		m.err = errCommentStorageUnavailable
		return
	}
	if err := delete(items[index], m.review.patch); err != nil {
		m.err = err
		return
	}
	items = append(items[:index], items[index+1:]...)
	comment.items = items
	comment.row = min(comment.row, max(0, len(items)-1))
	comment.revision++
	m.err = nil
}

func (m *Model) clearCommentEdit() {
	comment := &m.comments
	comment.body = ""
	comment.editIndex = -1
	comment.editAnchor = commentAnchor{}
}

func (m *Model) cancelSelection() { m.review.selection = nil }

func (m Model) openCurrentLine() (teaCmd, error) {
	review := m.review
	view, cursor := review.view, review.cursor
	editorCommand := trimSpace(os.Getenv("EDITOR"))
	if editorCommand == "" {
		return nil, errEditorUnset
	}
	file, fileOK := view.File(cursor)
	line, lineOK := view.Line(cursor)
	if !fileOK || !lineOK {
		return nil, formatError("select a code line to open in $EDITOR")
	}
	path, number := file.NewPath, line.NewNumber
	if missingPath(path) {
		path = file.OldPath
	}
	if number == 0 {
		number = line.OldNumber
	}
	if missingPath(path) || number < 1 {
		return nil, formatError("current line has no editable working-tree location")
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(review.patch.Repository, filepath.FromSlash(path))
	}
	return tea.ExecProcess(SourceCommand(editorCommand, path, int(number)), func(err error) teaMsg {
		return sourceEditorFinishedMsg{err: err}
	}), nil
}

func missingPath(path string) bool { return path == "" || path == "/dev/null" }

func (m Model) openCommentEditor() (teaCmd, error) {
	comment := m.comments
	anchor := comment.editAnchor
	editorCommand := trimSpace(os.Getenv("EDITOR"))
	if editorCommand == "" {
		return nil, errEditorUnset
	}
	path, err := CreateCommentFile(comment.body, anchor)
	if err != nil {
		return nil, err
	}
	return tea.ExecProcess(CommentCommand(editorCommand, path), func(editorErr error) teaMsg {
		body, err := ReadCommentFile(path, anchor, editorErr)
		return commentEditorFinishedMsg{body: body, err: err}
	}), nil
}
