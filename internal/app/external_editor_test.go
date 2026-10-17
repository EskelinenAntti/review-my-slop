package app

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	feedback "github.com/eskelinenantti/review-my-slop/internal/comments"
)

func TestEditorCommentWorkflowLifecycle(t *testing.T) {
	for _, tc := range []struct{ name, script, body, want, errorText string }{
		{"edited", `printf edited > "$1"`, "original", "edited", ""},
		{"unchanged suggestion", `true`, "original", "original", ""},
		{"failed process", `exit 7`, "original", "", "editor:"},
		{"failed read", `rm "$1"`, "original", "", "read comment file:"},
	} {
		t.Run(
			tc.name,
			func(t *testing.T) {
				dir := t.TempDir()
				record := filepath.Join(dir, "draft-path")
				script := filepath.Join(dir, "editor's script.sh")
				requireNoError(
					t,
					os.WriteFile(
						script,
						[]byte(
							"printf '%s' \"$1\" > "+(func(value string) string {
								return "'" + strings.ReplaceAll(
									value,
									"'",
									"'\"'\"'",
								) + "'"
							})(record)+
								"\n"+tc.script+"\n",
						),
						0600,
					),
				)

				adapter, complete := newTestEditorCommandAdapter(
					context.Background(),
					"sh "+(func(value string) string {
						return "'" + strings.ReplaceAll(
							value,
							"'",
							"'\"'\"'",
						) + "'"
					})(script),
					tc.body,
					feedback.Anchor{
						QuotedLines: []string{" context", "+added"},
					},
					func(body string, err error) tea.Msg {
						assertTestEditorCompletion(
							t, body, err, tc.want, tc.errorText,
						)
						return nil
					},
				)
				complete(adapter.Run())
				path, err := os.ReadFile(record)
				requireNoError(t, err)

				_, statErr := os.Stat(string(path))
				if !errors.Is(statErr, os.ErrNotExist) {
					t.Fatalf("draft still exists: %v", statErr)
				}
			},
		)
	}
}

func assertTestEditorCompletion(
	t *testing.T,
	body string,
	err error,
	want, errorText string,
) {
	t.Helper()
	checkEqual(t, body, want, "body=%q want=%q", body, want)
	if errorText == "" {
		requireNoError(t, err)
		return
	}
	requireEqual(t, err != nil, true,
		"expected error containing %q", errorText)
	checkEqual(t, strings.Contains(err.Error(), errorText), true,
		"error=%v", err)
}

func TestEditorUnusedCommandCreatesNoDraft(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TMPDIR", dir)
	t.Setenv("EDITOR", "true")
	cmd, err := (func(
		ctx context.Context,
		body string,
		anchor feedback.Anchor,
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
		adapter, complete := newTestEditorCommandAdapter(
			ctx,
			command,
			body,
			anchor,
			onComplete,
		)
		return tea.Exec(adapter, complete), nil
	})(
		context.Background(),
		"body",
		feedback.Anchor{},
		func(string, error) tea.Msg { return nil },
	)
	requireEqual(t, err, nil, "cmd=%v err=%v", cmd, err)
	requireEqual(t, cmd != nil, true, "cmd=%v err=%v", cmd, err)

	_ = cmd() // Producing Bubble Tea's execution message must also be lazy.
	files, err := os.ReadDir(dir)
	requireEqual(t, err, nil, "files=%v err=%v", files, err)
	requireEqual(t, len(files), 0, "files=%v err=%v", files, err)
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
		t.Run(
			tc.name,
			func(t *testing.T) {
				dir := t.TempDir()
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				tc.setup(t, dir, cancel)
				adapter, complete := newTestEditorCommandAdapter(
					ctx,
					"true",
					"body",
					feedback.Anchor{},
					func(
						body string,
						err error,
					) tea.Msg {
						requireEqual(t, err != nil, true, "body=%q err=%v",
							body, err,
						)
						requireEqual(t, body, "", "body=%q err=%v", body, err)
						return nil
					},
				)
				complete(adapter.Run())
				files, err := os.ReadDir(dir)
				requireNoError(t, err)
				requireEqual(t, len(files), 0, "files=%v", files)
			},
		)
	}
}

func TestEditorTerminalFailureReachesCompletion(t *testing.T) {
	sentinel := errors.New("terminal restoration failed")
	adapter, complete := newTestEditorCommandAdapter(
		context.Background(),
		"true",
		"body",
		feedback.Anchor{},
		func(
			body string,
			err error,
		) tea.Msg {
			requireEqual(t, body, "body", "body=%q err=%v", body, err)
			requireEqual(t, errors.Is(err, sentinel), true,
				"body=%q err=%v", body, err,
			)

			return nil
		},
	)
	requireNoError(t, adapter.Run())

	complete(sentinel)
}

func TestEditorMissingEditor(t *testing.T) {
	t.Setenv("EDITOR", " \t")
	cmd, err := newCommentEditorCommand(
		context.Background(), "", feedback.Anchor{}, nil,
	)
	requireEqual(t, cmd == nil, true, "cmd=%v err=%v", cmd, err)
	requireEqual(t, err != nil, true, "cmd=%v err=%v", cmd, err)
	model := reviewModel{}
	cmd, err = model.startSourceEditor(
		context.Background(), "source", 2, nil,
	)
	requireEqual(t, cmd == nil, true, "cmd=%v err=%v", cmd, err)
	requireEqual(t, err != nil, true, "cmd=%v err=%v", cmd, err)
}

func TestEditorSourceArgumentsAndContext(t *testing.T) {
	var output bytes.Buffer
	cmd := exec.CommandContext(
		context.Background(),
		"sh",
		"-c",
		"printf '%s\\n'"+" +"+strconv.Itoa(42)+" "+(func(value string) string {
			return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
		})(
			"/repo with spaces/it's.go",
		),
	)
	cmd.Stdout = &output
	requireNoError(t, cmd.Run())

	if got := output.String(); got != "+42\n/repo with spaces/it's.go\n" {
		t.Fatalf("arguments = %q", got)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := exec.CommandContext(ctx, "sh", "-c", "true"+" +"+strconv.Itoa(
		2,
	)+" "+(func(value string) string {
		return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
	})(
		"source",
	)).Run()
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
		anchor feedback.Anchor,
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
		adapter, complete := newTestEditorCommandAdapter(
			ctx,
			command,
			body,
			anchor,
			onComplete,
		)
		return tea.Exec(adapter, complete), nil
	})(
		context.Background(),
		"original",
		feedback.Anchor{},
		func(
			body string,
			err error,
		) tea.Msg {
			result = editorCompletionMsg{body, err}
			called = true
			return result
		},
	)
	requireNoError(t, err)

	program := tea.NewProgram(
		editorWorkflowModel{cmd},
		tea.WithInput(nil),
		tea.WithOutput(&bytes.Buffer{}),
		tea.WithoutRenderer(),
	)
	if _, err := program.Run(); err != nil {
		t.Fatal(err)
	}
	requireEqual(t, called, true, "called=%v result=%#v", called, result)
	requireEqual(t, result.body, "edited", "called=%v result=%#v",
		called, result,
	)
	requireEqual(t, result.err, nil, "called=%v result=%#v", called, result)
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
	adapter, complete := newTestEditorCommandAdapter(
		ctx,
		"printf started; exec sleep 30 #",
		"body",
		feedback.Anchor{},
		func(
			body string,
			err error,
		) tea.Msg {
			requireEqual(t, err != nil, true, "body=%q error=%v", body, err)
			requireEqual(t, body, "", "body=%q error=%v", body, err)

			return nil
		},
	)
	adapter.SetStdout(editorCancelOnWrite{cancel})
	complete(adapter.Run())
	files, err := os.ReadDir(dir)
	requireEqual(t, err, nil, "files=%v error=%v", files, err)
	requireEqual(t, len(files), 0, "files=%v error=%v", files, err)
}
