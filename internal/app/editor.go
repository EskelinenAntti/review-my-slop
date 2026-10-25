package app

import (
	"errors"
	"path/filepath"

	tea "charm.land/bubbletea/v2"

	"github.com/eskelinenantti/review-my-slop/internal/comments"
	"github.com/eskelinenantti/review-my-slop/internal/editor"
)

type commentEdit struct {
	body   string
	index  int
	anchor comments.Anchor
}

type commentEditorFinishedMsg struct {
	body string
	err  error
}

type sourceEditorFinishedMsg struct{ err error }

func (m *model) startEditor(
	start func() (tea.Cmd, error),
) (tea.Model, tea.Cmd) {
	cmd, err := start()
	if err != nil {
		m.err = err
		return *m, nil
	}
	return *m, cmd
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
	anchor := comments.NewAnchor(file, lines)
	m.edit = commentEdit{index: -1, anchor: anchor}
	return m.openCommentEditor()
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

func (m *model) finishCommentEditor(msg commentEditorFinishedMsg) {
	if msg.err != nil {
		m.err = msg.err
		m.clearCommentEdit()
		return
	}
	m.edit.body = msg.body
	m.finishCommentEdit()
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
