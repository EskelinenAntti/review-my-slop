package main

import (
	bytes "bytes"
	context "context"
	errors "errors"
	fmt "fmt"
	os "os"
	exec "os/exec"
	filepath "path/filepath"
	strconv "strconv"
	strings "strings"
	testing "testing"

	tea "charm.land/bubbletea/v2"
)

func TestEditorCommentWorkflowLifecycle(t *testing.T) {
	for _, tc := range []struct{ name, script, body, want, errorText string }{
		{"edited", `printf edited > "$1"`, "original", "edited", ""},
		{"unchanged suggestion", `true`, "original", "original", ""},
		{"failed process", `exit 7`, "original", "", "editor:"},
		{"failed read", `rm "$1"`, "original", "", "read comment file:"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			record := filepath.Join(dir, "draft-path")
			script := filepath.Join(dir, "editor's script.sh")
			(func(t *testing.T, err error) {
				t.Helper()
				if err != nil {
					t.Fatal(err)
				}
			})(
				t,
				os.WriteFile(
					script,
					[]byte(
						"printf '%s' \"$1\" > "+(func(value string) string {
							return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
						})(record)+
							"\n"+tc.script+"\n",
					),
					0600,
				),
			)

			adapter, complete := (func(
				ctx context.Context,
				command, body string,
				anchor commentsAnchor,
				onComplete func(string, error) tea.Msg,
			) (*editorCommandAdapter, tea.ExecCallback) {
				adapter := &editorCommandAdapter{}
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
			})(
				context.Background(),
				"sh "+(func(value string) string {
					return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
				})(script),
				tc.body, commentsAnchor{
					QuotedLines: []string{" context", "+added"},
				}, func(body string, err error) tea.Msg {
					(func(
						t *testing.T,
						body string,
						err error,
						want, errorText string,
					) {
						t.Helper()
						(func(t *testing.T, got, want string, format string, args ...any) {
							t.Helper()
							if got != want {
								t.Errorf(format, args...)
							}
						})(t, body, want, "body=%q want=%q", body, want)
						if errorText == "" {
							(func(t *testing.T, err error) {
								t.Helper()
								if err != nil {
									t.Fatal(err)
								}
							})(t, err)
							return
						}
						(func(t *testing.T, got, want bool, format string, args ...any) {
							t.Helper()
							if got != want {
								t.Fatalf(format, args...)
							}
						})(t, err != nil, true, "expected error containing %q",
							errorText,
						)
						(func(t *testing.T, got, want bool, format string, args ...any) {
							t.Helper()
							if got != want {
								t.Errorf(format, args...)
							}
						})(t, strings.Contains(err.Error(), errorText), true,
							"error=%v", err,
						)
					})(t, body, err, tc.want, tc.errorText)
					return nil
				},
			)
			complete(adapter.Run())
			path, err := os.ReadFile(record)
			(func(t *testing.T, err error) {
				t.Helper()
				if err != nil {
					t.Fatal(err)
				}
			})(t, err)

			_, statErr := os.Stat(string(path))
			if !errors.Is(statErr, os.ErrNotExist) {
				t.Fatalf("draft still exists: %v", statErr)
			}
		})
	}
}

func TestEditorUnusedCommandCreatesNoDraft(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TMPDIR", dir)
	t.Setenv("EDITOR", "true")
	cmd, err := (func(
		ctx context.Context,
		body string,
		anchor commentsAnchor,
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
			anchor commentsAnchor,
			onComplete func(string, error) tea.Msg,
		) (*editorCommandAdapter, tea.ExecCallback) {
			adapter := &editorCommandAdapter{}
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
	})(
		context.Background(),
		"body", commentsAnchor{}, func(string, error) tea.Msg { return nil },
	)
	(func(t *testing.T, got, want error, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, err, nil, "cmd=%v err=%v", cmd, err)
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, cmd != nil, true, "cmd=%v err=%v", cmd, err)

	_ = cmd() // Producing Bubble Tea's execution message must also be lazy.
	files, err := os.ReadDir(dir)
	(func(t *testing.T, got, want error, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, err, nil, "files=%v err=%v", files, err)
	(func(t *testing.T, got, want int, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, len(files), 0, "files=%v err=%v", files, err)
}

func TestEditorCommentCancellationAndCreationFailure(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup func(*testing.T, string, context.CancelFunc)
	}{
		{
			"creation failure",
			func(t *testing.T, dir string, _ context.CancelFunc) {
				t.Setenv("TMPDIR", filepath.Join(dir, "missing"))
			},
		},
		{
			"cancellation",
			func(
				t *testing.T,
				dir string,
				cancel context.CancelFunc,
			) {
				cancel()
				t.Setenv("TMPDIR", dir)
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			tc.setup(t, dir, cancel)
			adapter, complete := (func(
				ctx context.Context,
				command, body string,
				anchor commentsAnchor,
				onComplete func(string, error) tea.Msg,
			) (*editorCommandAdapter, tea.ExecCallback) {
				adapter := &editorCommandAdapter{}
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
			})(
				ctx,
				"true",
				"body", commentsAnchor{}, func(
					body string,
					err error,
				) tea.Msg {
					(func(t *testing.T, got, want bool, format string, args ...any) {
						t.Helper()
						if got != want {
							t.Fatalf(format, args...)
						}
					})(t, err != nil, true, "body=%q err=%v",
						body, err,
					)
					(func(t *testing.T, got, want string, format string, args ...any) {
						t.Helper()
						if got != want {
							t.Fatalf(format, args...)
						}
					})(t, body, "", "body=%q err=%v", body,
						err,
					)
					return nil
				},
			)
			complete(adapter.Run())
			files, err := os.ReadDir(dir)
			(func(t *testing.T, err error) {
				t.Helper()
				if err != nil {
					t.Fatal(err)
				}
			})(t, err)
			(func(t *testing.T, got, want int, format string, args ...any) {
				t.Helper()
				if got != want {
					t.Fatalf(format, args...)
				}
			})(t, len(files), 0, "files=%v", files)
		})
	}
}

func TestEditorTerminalFailureReachesCompletion(t *testing.T) {
	sentinel := errors.New("terminal restoration failed")
	adapter, complete := (func(
		ctx context.Context,
		command, body string,
		anchor commentsAnchor,
		onComplete func(string, error) tea.Msg,
	) (*editorCommandAdapter, tea.ExecCallback) {
		adapter := &editorCommandAdapter{}
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
	})(
		context.Background(),
		"true",
		"body", commentsAnchor{}, func(
			body string,
			err error,
		) tea.Msg {
			(func(t *testing.T, got, want string, format string, args ...any) {
				t.Helper()
				if got != want {
					t.Fatalf(format, args...)
				}
			})(t, body, "body", "body=%q err=%v", body, err)
			(func(t *testing.T, got, want bool, format string, args ...any) {
				t.Helper()
				if got != want {
					t.Fatalf(format, args...)
				}
			})(t, errors.Is(err, sentinel), true,
				"body=%q err=%v", body, err,
			)

			return nil
		},
	)
	(func(t *testing.T, err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	})(t, adapter.Run())

	complete(sentinel)
}

func TestEditorMissingEditor(t *testing.T) {
	t.Setenv("EDITOR", " \t")
	cmd, err := (func(
		ctx context.Context,
		body string,
		anchor commentsAnchor,
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
			anchor commentsAnchor,
			onComplete func(string, error) tea.Msg,
		) (*editorCommandAdapter, tea.ExecCallback) {
			adapter := &editorCommandAdapter{}
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
	})(context.Background(), "", commentsAnchor{}, nil)
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, cmd == nil, true, "cmd=%v err=%v", cmd, err)
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, err != nil, true, "cmd=%v err=%v", cmd, err)
	cmd, err = (func(
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
		adapter := &editorCommandAdapter{}
		adapter.run = func() error {
			var cmd *exec.Cmd = exec.CommandContext(ctx, "sh", "-c", command+" +"+strconv.Itoa(line)+" "+(func(value string) string {
				return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
			})(path))
			cmd.Stdin, cmd.Stdout, cmd.Stderr = adapter.stdin, adapter.stdout, adapter.stderr
			return cmd.Run()
		}
		return tea.Exec(adapter, onComplete), nil
	})(context.Background(), "source", 2, nil)
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, cmd == nil, true, "cmd=%v err=%v", cmd, err)
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, err != nil, true, "cmd=%v err=%v", cmd, err)
}

func TestEditorSourceArgumentsAndContext(t *testing.T) {
	var output bytes.Buffer
	cmd := exec.CommandContext(context.Background(), "sh", "-c", "printf '%s\\n'"+" +"+strconv.Itoa(42)+" "+(func(value string) string {
		return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
	})("/repo with spaces/it's.go"))
	cmd.Stdout = &output
	(func(t *testing.T, err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	})(t, cmd.Run())

	if got := output.String(); got != "+42\n/repo with spaces/it's.go\n" {
		t.Fatalf("arguments = %q", got)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := exec.CommandContext(ctx, "sh", "-c", "true"+" +"+strconv.Itoa(2)+" "+(func(value string) string {
		return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
	})("source")).Run()
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v", err)
	}
}

type editorCompletionMsg struct {
	body string
	err  error
}
type editorWorkflowModel struct{ command tea.Cmd }

func (m editorWorkflowModel) Init() tea.Cmd { return m.command }
func (m editorWorkflowModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if _, ok := msg.(editorCompletionMsg); ok {
		return m, tea.Quit
	}
	return m, nil
}
func (editorWorkflowModel) View() tea.View { return tea.NewView("") }

func TestEditorEditCommentThroughBubbleTea(t *testing.T) {
	t.Setenv("EDITOR", "printf edited >")
	var result editorCompletionMsg
	called := false
	cmd, err := (func(
		ctx context.Context,
		body string,
		anchor commentsAnchor,
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
			anchor commentsAnchor,
			onComplete func(string, error) tea.Msg,
		) (*editorCommandAdapter, tea.ExecCallback) {
			adapter := &editorCommandAdapter{}
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
	})(
		context.Background(),
		"original", commentsAnchor{}, func(
			body string,
			err error,
		) tea.Msg {
			result = editorCompletionMsg{body, err}
			called = true
			return result
		},
	)
	(func(t *testing.T, err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	})(t, err)

	program := tea.NewProgram(
		editorWorkflowModel{cmd},
		tea.WithInput(nil),
		tea.WithOutput(&bytes.Buffer{}),
		tea.WithoutRenderer(),
	)
	if _, err := program.Run(); err != nil {
		t.Fatal(err)
	}
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, called, true, "called=%v result=%#v", called,
		result,
	)
	(func(t *testing.T, got, want string, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, result.body, "edited", "called=%v result=%#v",
		called, result,
	)
	(func(t *testing.T, got, want error, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, result.err, nil, "called=%v result=%#v", called,
		result,
	)
}

type editorCancelOnWrite struct{ cancel context.CancelFunc }

func (w editorCancelOnWrite) Write(p []byte) (int, error) {
	w.cancel()
	return len(p), nil
}

func TestEditorRunningEditorCancellationCleansDraft(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TMPDIR", dir)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	adapter, complete := (func(
		ctx context.Context,
		command, body string,
		anchor commentsAnchor,
		onComplete func(string, error) tea.Msg,
	) (*editorCommandAdapter, tea.ExecCallback) {
		adapter := &editorCommandAdapter{}
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
	})(
		ctx,
		"printf started; exec sleep 30 #",
		"body", commentsAnchor{}, func(
			body string,
			err error,
		) tea.Msg {
			(func(t *testing.T, got, want bool, format string, args ...any) {
				t.Helper()
				if got != want {
					t.Fatalf(format, args...)
				}
			})(t, err != nil, true, "body=%q error=%v", body,
				err,
			)
			(func(t *testing.T, got, want string, format string, args ...any) {
				t.Helper()
				if got != want {
					t.Fatalf(format, args...)
				}
			})(t, body, "", "body=%q error=%v", body, err)

			return nil
		},
	)
	adapter.SetStdout(editorCancelOnWrite{cancel})
	complete(adapter.Run())
	files, err := os.ReadDir(dir)
	(func(t *testing.T, got, want error, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, err, nil, "files=%v error=%v", files, err)
	(func(t *testing.T, got, want int, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, len(files), 0, "files=%v error=%v", files, err)
}
