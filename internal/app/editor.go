package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	feedback "github.com/eskelinenantti/review-my-slop/internal/comments"
)

type editorCommandAdapter struct {
	run            func() error
	body           string
	stdin          io.Reader
	stdout, stderr io.Writer
}

func (c *editorCommandAdapter) Run() error { return c.run() }

func (c *editorCommandAdapter) SetStdin(r io.Reader) { c.stdin = r }

func (c *editorCommandAdapter) SetStdout(w io.Writer) { c.stdout = w }

func (c *editorCommandAdapter) SetStderr(w io.Writer) { c.stderr = w }

func newCommentEditorCommand(
	ctx context.Context,
	body string,
	anchor feedback.Anchor,
	onComplete func(string, error) tea.Msg,
) (tea.Cmd, error) {
	command, err := configuredEditorCommand()
	if err != nil {
		return nil, err
	}
	adapter := &editorCommandAdapter{}
	adapter.run = func() error {
		path, err := createCommentDraft(body, anchor)
		if err != nil {
			return err
		}
		defer os.Remove(path)
		if err := runCommentEditor(ctx, command, path, adapter); err != nil {
			return fmt.Errorf("editor: %w", err)
		}
		adapter.body, err = readCommentDraft(path, anchor)
		return err
	}
	complete := func(err error) tea.Msg { return onComplete(adapter.body, err) }
	return tea.Exec(adapter, complete), nil
}

func configuredEditorCommand() (string, error) {
	command := strings.TrimSpace(os.Getenv("EDITOR"))
	if command == "" {
		return "", errors.New("$EDITOR is not set")
	}
	return command, nil
}

func createCommentDraft(body string, anchor feedback.Anchor) (string, error) {
	file, err := os.CreateTemp("", "review-my-slop-comment-*.md")
	if err != nil {
		return "", fmt.Errorf("create comment file: %w", err)
	}
	path := file.Name()
	if _, err := file.WriteString(commentDraftText(body, anchor)); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return "", fmt.Errorf("write comment file: %w", err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return "", fmt.Errorf("close comment file: %w", err)
	}
	return path, nil
}

func commentDraftText(body string, anchor feedback.Anchor) string {
	if len(anchor.QuotedLines) == 0 {
		return body
	}
	lines := commentSuggestionLines(anchor)
	fence := suggestionFence(lines)
	var draft strings.Builder
	_, _ = draft.WriteString(body)
	if body != "" && !strings.HasSuffix(body, "\n") {
		_ = draft.WriteByte('\n')
	}
	_ = draft.WriteByte('\n')
	_, _ = draft.WriteString(fence + "suggestion\n")
	for _, line := range lines {
		_, _ = draft.WriteString(line)
		_ = draft.WriteByte('\n')
	}
	_, _ = draft.WriteString(fence)
	_ = draft.WriteByte('\n')
	return draft.String()
}

func commentSuggestionLines(anchor feedback.Anchor) []string {
	lines := make([]string, 0, len(anchor.QuotedLines))
	for _, line := range anchor.QuotedLines {
		if line != "" && line[0] != '-' {
			lines = append(lines, line[1:])
		}
	}
	return lines
}

func suggestionFence(lines []string) string {
	longest := 0
	for _, line := range lines {
		run := 0
		for _, char := range line {
			if char == '`' {
				run++
				longest = max(longest, run)
			} else {
				run = 0
			}
		}
	}
	return strings.Repeat("`", max(3, longest+1))
}

func readCommentDraft(path string, anchor feedback.Anchor) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read comment file: %w", err)
	}
	if len(anchor.QuotedLines) == 0 {
		return string(data), nil
	}
	return stripCommentSuggestion(string(data), anchor), nil
}

func stripCommentSuggestion(body string, anchor feedback.Anchor) string {
	lines := commentSuggestionLines(anchor)
	fence := suggestionFence(lines)
	suggestion := fence + "suggestion\n" + strings.Join(
		lines,
		"\n",
	) + "\n" + fence
	start := strings.Index(body, suggestion)
	if start < 0 {
		return body
	}
	end := start + len(suggestion)
	before := strings.TrimRight(body[:start], "\n")
	after := body[end:]
	if strings.TrimSpace(after) == "" {
		return before
	}
	return before + "\n" + strings.TrimLeft(after, "\n")
}

func runCommentEditor(
	ctx context.Context,
	command, path string,
	adapter *editorCommandAdapter,
) error {
	quotedPath := "'" + strings.ReplaceAll(path, "'", "'\"'\"'") + "'"
	cmd := exec.CommandContext(ctx, "sh", "-c", command+" "+quotedPath)
	cmd.Stdin = adapter.stdin
	cmd.Stdout = adapter.stdout
	cmd.Stderr = adapter.stderr
	return cmd.Run()
}

func editedSourceLocation(m reviewModel) (string, int, error) {
	file, line, ok := m.diffView.Current()
	if !ok {
		return "", 0, errors.New("select a code line to open in $EDITOR")
	}
	path, number := file.NewPath, line.NewNumber
	if path == "" || path == "/dev/null" {
		path = file.OldPath
	}
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

func sourceEditorCommand(
	ctx context.Context,
	command, path string,
	line int,
	adapter *editorCommandAdapter,
) error {
	quotedPath := "'" + strings.ReplaceAll(path, "'", "'\"'\"'") + "'"
	cmd := exec.CommandContext(ctx, "sh", "-c", command+" +"+strconv.Itoa(
		line,
	)+" "+quotedPath)
	cmd.Stdin = adapter.stdin
	cmd.Stdout = adapter.stdout
	cmd.Stderr = adapter.stderr
	return cmd.Run()
}

func (reviewModel) startSourceEditor(
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
		return sourceEditorCommand(ctx, command, path, line, adapter)
	}
	return tea.Exec(adapter, onComplete), nil
}
