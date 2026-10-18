package editor

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"
)

// EditComment edits a private Markdown draft and returns the resulting body.
// Quoted lines include diff prefixes and seed an editable suggestion block.
func EditComment(
	ctx context.Context,
	body string,
	quoted []string,
	onComplete func(string, error) tea.Msg,
) (tea.Cmd, error) {
	command, err := editorCommand()
	if err != nil {
		return nil, err
	}
	adapter, complete := commentWorkflow(ctx, command, body, quoted, onComplete)
	return tea.Exec(adapter, complete), nil
}

func commentWorkflow(
	ctx context.Context,
	command, body string,
	quoted []string,
	onComplete func(string, error) tea.Msg,
) (*commandAdapter, tea.ExecCallback) {
	adapter := &commandAdapter{}
	var editedBody string
	adapter.run = func() error {
		path, err := createCommentFile(body, quoted)
		if err != nil {
			return err
		}
		defer os.Remove(path)
		editorCommand := commentCommand(ctx, command, path)
		if err := adapter.execute(editorCommand); err != nil {
			return fmt.Errorf("editor: %w", err)
		}
		editedBody, err = readCommentFile(path, quoted)
		return err
	}
	return adapter, func(err error) tea.Msg {
		return onComplete(editedBody, err)
	}
}

// OpenSource opens a resolved source path at the supplied line.
func OpenSource(
	ctx context.Context,
	path string,
	line int,
	onComplete func(error) tea.Msg,
) (tea.Cmd, error) {
	command, err := editorCommand()
	if err != nil {
		return nil, err
	}
	adapter := &commandAdapter{}
	adapter.run = func() error {
		return adapter.execute(sourceCommand(ctx, command, path, line))
	}
	return tea.Exec(adapter, onComplete), nil
}

func editorCommand() (string, error) {
	command := strings.TrimSpace(os.Getenv("EDITOR"))
	if command == "" {
		return "", errors.New("$EDITOR is not set")
	}
	return command, nil
}
