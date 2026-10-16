package editor

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/eskelinenantti/review-my-slop/internal/comments"
)

// EditComment edits a private Markdown draft and returns the resulting body.
func EditComment(
	ctx context.Context,
	body string,
	anchor comments.Anchor,
	onComplete func(string, error) tea.Msg,
) (tea.Cmd, error) {
	command, err := editorCommand()
	if err != nil {
		return nil, err
	}
	adapter, complete := newCommentEditorWorkflow(ctx, command, body, anchor, onComplete)
	return tea.Exec(adapter, complete), nil
}

func newCommentEditorWorkflow(
	ctx context.Context,
	command, body string,
	anchor comments.Anchor,
	onComplete func(string, error) tea.Msg,
) (editorCommand *commandAdapter, execCallback tea.ExecCallback) {
	adapter := &commandAdapter{}
	adapter.run = func() error {
		path, err := createTemporaryCommentFile(body, anchor)
		if err != nil {
			return err
		}
		defer os.Remove(path)
		editorCommand := newCommentEditorCommand(ctx, command, path)
		if err := adapter.execute(editorCommand); err != nil {
			return fmt.Errorf("editor: %w", err)
		}
		adapter.body, err = readEditedComment(path, anchor)
		return err
	}
	return adapter, func(err error) tea.Msg {
		return onComplete(adapter.body, err)
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
		return adapter.execute(newSourceEditorCommand(ctx, command, path, line))
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

type commandAdapter struct {
	run            func() error
	body           string
	stdin          io.Reader
	stdout, stderr io.Writer
}

func (c *commandAdapter) Run() error            { return c.run() }
func (c *commandAdapter) SetStdin(r io.Reader)  { c.stdin = r }
func (c *commandAdapter) SetStdout(w io.Writer) { c.stdout = w }
func (c *commandAdapter) SetStderr(w io.Writer) { c.stderr = w }
func (c *commandAdapter) execute(cmd *exec.Cmd) error {
	cmd.Stdin, cmd.Stdout, cmd.Stderr = c.stdin, c.stdout, c.stderr
	return cmd.Run()
}
