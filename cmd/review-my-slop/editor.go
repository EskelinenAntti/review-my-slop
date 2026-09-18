package main

import (
	"fmt"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/eskelinenantti/review-my-slop/internal/comment"
	"github.com/eskelinenantti/review-my-slop/internal/store"
	"github.com/eskelinenantti/review-my-slop/internal/ui"
)

type systemEditor struct{}

func (systemEditor) EditComment(body string, anchor comment.Anchor) (tea.Cmd, error) {
	editor, err := editorCommand()
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
	return tea.ExecProcess(comment.CommentCommand(editor, path), func(editorErr error) tea.Msg {
		body, err := comment.ReadDraft(path, anchor, editorErr)
		return ui.CommentEditedMsg{Body: body, Err: err}
	}), nil
}

func (systemEditor) OpenSource(path string, line int) (tea.Cmd, error) {
	editor, err := editorCommand()
	if err != nil {
		return nil, err
	}
	return tea.ExecProcess(comment.SourceCommand(editor, path, line), func(err error) tea.Msg {
		return ui.SourceEditedMsg{Err: err}
	}), nil
}

func editorCommand() (string, error) {
	editor := strings.TrimSpace(os.Getenv("EDITOR"))
	if editor == "" {
		return "", fmt.Errorf("$EDITOR is not set")
	}
	return editor, nil
}
