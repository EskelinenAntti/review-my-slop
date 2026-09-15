package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/eskelinenantti/review-my-slop/internal/comment"
	"github.com/eskelinenantti/review-my-slop/internal/store"
)

type Editor interface {
	EditComment(body string, anchor comment.Anchor) (tea.Cmd, error)
	OpenSource(path string, line int) (tea.Cmd, error)
}

type SystemEditor struct{}

func (SystemEditor) EditComment(body string, anchor comment.Anchor) (tea.Cmd, error) {
	editorCommand, err := editorCommand()
	if err != nil {
		return nil, err
	}
	state, err := store.StateDir()
	if err != nil {
		return nil, err
	}
	path, err := comment.CreateDraft(state, body, anchor)
	if err != nil {
		return nil, err
	}
	return tea.ExecProcess(comment.CommentCommand(editorCommand, path), func(editorErr error) tea.Msg {
		body, err := comment.ReadDraft(path, anchor, editorErr)
		return CommentEditedMsg{Body: body, Err: err}
	}), nil
}

func (SystemEditor) OpenSource(path string, line int) (tea.Cmd, error) {
	editorCommand, err := editorCommand()
	if err != nil {
		return nil, err
	}
	return tea.ExecProcess(comment.SourceCommand(editorCommand, path, line), func(err error) tea.Msg {
		return SourceEditedMsg{Err: err}
	}), nil
}

func editorCommand() (string, error) {
	editor := strings.TrimSpace(os.Getenv("EDITOR"))
	if editor == "" {
		return "", fmt.Errorf("$EDITOR is not set")
	}
	return editor, nil
}

func (m Model) openCurrentLine() (tea.Cmd, error) {
	file, fileOK := m.changes.view.File(m.changes.cursor)
	line, lineOK := m.changes.view.Line(m.changes.cursor)
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
		path = filepath.Join(m.changes.changes.Repository, filepath.FromSlash(path))
	}
	editor := m.dependencies.Editor
	if editor == nil {
		editor = SystemEditor{}
	}
	return editor.OpenSource(path, int(number))
}
