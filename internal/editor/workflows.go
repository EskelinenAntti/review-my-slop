package editor

import (
	tea "charm.land/bubbletea/v2"
	"context"
	"errors"
	"fmt"
	"github.com/eskelinenantti/review-my-slop/internal/comments"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// EditComment edits a private Markdown draft and returns the resulting body.
func EditComment(
	ctx context.Context,
	body string,
	anchor comments.Anchor,
	onComplete func(string, error) tea.Msg,
) (tea.Cmd, error) {
	command, err := (func() (string, error) {
		command := strings.TrimSpace(os.Getenv("EDITOR"))
		if command == "" {
			return "", errors.New("$EDITOR is not set")
		}
		return command, nil
	})()
	if err != nil {
		return nil, err
	}
	adapter, complete := (func(
		ctx context.Context,
		command, body string,
		anchor comments.Anchor,
		onComplete func(string, error) tea.Msg,
	) (*commandAdapter, tea.ExecCallback) {
		adapter := &commandAdapter{}
		adapter.run = func() error {
			path, err := func() (string, error) {
				file, err := os.CreateTemp("", "review-my-slop-comment-*.md")
				if err != nil {
					return "", fmt.Errorf("create comment file: %w", err)
				}
				path := file.Name()
				if _, err := file.WriteString(func() string {
					if len(anchor.QuotedLines) == 0 {
						return body
					}
					lines := func() []string {
						lines := make([]string, 0, len(anchor.QuotedLines))
						for _, line := range anchor.QuotedLines {
							if line != "" && line[0] != '-' {
								lines = append(lines, line[1:])
							}
						}
						return lines
					}()
					var draft strings.Builder
					_, _ = draft.WriteString(body)
					if body != "" && !strings.HasSuffix(body, "\n") {
						_ = draft.WriteByte('\n')
					}
					_ = draft.WriteByte('\n')
					fence := (func(lines []string) string {
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
					})(lines)
					_, _ = draft.WriteString(fence)
					_, _ = draft.WriteString("suggestion\n")
					for _, line := range lines {
						_, _ = draft.WriteString(line)
						_ = draft.WriteByte('\n')
					}
					_, _ = draft.WriteString(fence)
					_ = draft.WriteByte('\n')
					return draft.String()
				}()); err != nil {
					_ = file.Close()
					_ = os.Remove(path)
					return "", fmt.Errorf("write comment file: %w", err)
				}
				if err := file.Close(); err != nil {
					_ = os.Remove(path)
					return "", fmt.Errorf("close comment file: %w", err)
				}
				return path, nil
			}()
			if err != nil {
				return err
			}
			defer os.Remove(path)
			editorCommand := exec.CommandContext(ctx, "sh", "-c", command+" "+(func(value string) string {
				return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
			})(path))
			if err := func() error {
				editorCommand.Stdin, editorCommand.Stdout, editorCommand.Stderr = adapter.stdin, adapter.stdout, adapter.stderr
				return editorCommand.Run()
			}(); err != nil {
				return fmt.Errorf("editor: %w", err)
			}
			adapter.body, err = func() (string, error) {
				body, err := os.ReadFile(path)
				if err != nil {
					return "", fmt.Errorf("read comment file: %w", err)
				}
				return func() string {
					if len(anchor.QuotedLines) == 0 {
						return string(body)
					}
					lines := func() []string {
						lines := make([]string, 0, len(anchor.QuotedLines))
						for _, line := range anchor.QuotedLines {
							if line != "" && line[0] != '-' {
								lines = append(lines, line[1:])
							}
						}
						return lines
					}()
					fence := (func(lines []string) string {
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
					})(lines)
					var suggestion strings.Builder
					_, _ = suggestion.WriteString(fence)
					_, _ = suggestion.WriteString("suggestion\n")
					for _, line := range lines {
						_, _ = suggestion.WriteString(line)
						_ = suggestion.WriteByte('\n')
					}
					_, _ = suggestion.WriteString(fence)
					start := strings.Index(string(body), suggestion.String())
					if start < 0 {
						return string(body)
					}
					end := start + suggestion.Len()
					before := strings.TrimRight(string(body)[:start], "\n")
					after := string(body)[end:]
					if strings.TrimSpace(after) == "" {
						return before
					}
					return before + "\n" + strings.TrimLeft(after, "\n")
				}(), nil
			}()
			return err
		}
		return adapter, func(err error) tea.Msg {
			return onComplete(adapter.body, err)
		}
	})(ctx, command, body, anchor, onComplete)
	return tea.Exec(adapter, complete), nil
}

// OpenSource opens a resolved source path at the supplied line.
func OpenSource(
	ctx context.Context,
	path string,
	line int,
	onComplete func(error) tea.Msg,
) (tea.Cmd, error) {
	command, err := (func() (string, error) {
		command := strings.TrimSpace(os.Getenv("EDITOR"))
		if command == "" {
			return "", errors.New("$EDITOR is not set")
		}
		return command, nil
	})()
	if err != nil {
		return nil, err
	}
	adapter := &commandAdapter{}
	adapter.run = func() error {
		var cmd *exec.Cmd = exec.CommandContext(ctx, "sh", "-c", command+" +"+strconv.Itoa(line)+" "+(func(value string) string {
			return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
		})(path))
		cmd.Stdin, cmd.Stdout, cmd.Stderr = adapter.stdin, adapter.stdout, adapter.stderr
		return cmd.Run()
	}
	return tea.Exec(adapter, onComplete), nil
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
