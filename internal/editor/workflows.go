package editor

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"

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
	command, err := configuredEditorCommand()
	if err != nil {
		return nil, err
	}
	adapter, complete := newCommentEditorWorkflow(
		ctx,
		command,
		commentDraftInput{
			body:   body,
			anchor: anchor,
		},
		onComplete,
	)
	return tea.Exec(adapter, complete), nil
}

// OpenSource opens a resolved source path at the supplied line.
func OpenSource(
	ctx context.Context,
	path string,
	line int,
	onComplete func(error) tea.Msg,
) (tea.Cmd, error) {
	command, err := configuredEditorCommand()
	if err != nil {
		return nil, err
	}
	adapter := &editorCommandAdapter{}
	adapter.run = func() error {
		return adapter.execute(newSourceEditorCommand(ctx, command, path, line))
	}
	return tea.Exec(adapter, onComplete), nil
}

type commentDraftInput struct {
	body   string
	anchor comments.Anchor
}

func newCommentEditorWorkflow(
	ctx context.Context,
	command string,
	draft commentDraftInput,
	onComplete func(string, error) tea.Msg,
) (*editorCommandAdapter, tea.ExecCallback) {
	adapter := &editorCommandAdapter{}
	adapter.run = func() error {
		path, err := writeCommentDraftFile(draft.body, draft.anchor)
		if err != nil {
			return err
		}
		defer os.Remove(path)
		editorCommand := newCommentEditorCommand(ctx, command, path)
		if err := adapter.execute(editorCommand); err != nil {
			return fmt.Errorf("editor: %w", err)
		}
		adapter.body, err = readEditedCommentDraft(path, draft.anchor)
		return err
	}
	return adapter, func(err error) tea.Msg {
		return onComplete(adapter.body, err)
	}
}

type editorCommandAdapter struct {
	run            func() error
	body           string
	stdin          io.Reader
	stdout, stderr io.Writer
}

func (c *editorCommandAdapter) Run() error            { return c.run() }
func (c *editorCommandAdapter) SetStdin(r io.Reader)  { c.stdin = r }
func (c *editorCommandAdapter) SetStdout(w io.Writer) { c.stdout = w }
func (c *editorCommandAdapter) SetStderr(w io.Writer) { c.stderr = w }
func (c *editorCommandAdapter) execute(cmd *exec.Cmd) error {
	cmd.Stdin, cmd.Stdout, cmd.Stderr = c.stdin, c.stdout, c.stderr
	return cmd.Run()
}
