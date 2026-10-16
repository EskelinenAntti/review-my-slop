package editor

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/eskelinenantti/review-my-slop/internal/comments"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestCommentWorkflowLifecycle(t *testing.T) {
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
			testutil.NoError(
				t,
				os.WriteFile(
					script,
					[]byte(
						"printf '%s' \"$1\" > "+quoteShellArgument(record)+
							"\n"+tc.script+"\n",
					),
					0600,
				),
			)

			adapter, complete := newCommentEditorWorkflow(
				context.Background(),
				"sh "+quoteShellArgument(script),
				tc.body,
				comments.Anchor{
					QuotedLines: []string{" context", "+added"},
				},
				func(body string, err error) tea.Msg {
					assertWorkflowResult(t, body, err, tc.want, tc.errorText)
					return nil
				},
			)
			complete(adapter.Run())
			path, err := os.ReadFile(record)
			testutil.NoError(t, err)

			_, statErr := os.Stat(string(path))
			if !errors.Is(statErr, os.ErrNotExist) {
				t.Fatalf("draft still exists: %v", statErr)
			}
		})
	}
}

func assertWorkflowResult(
	t *testing.T,
	body string,
	err error,
	want, errorText string,
) {
	t.Helper()
	testutil.CheckEqual(t, body, want, "body=%q want=%q", body, want)
	if errorText == "" {
		testutil.NoError(t, err)
		return
	}
	testutil.RequireEqual(t, err != nil, true, "expected error containing %q",
		errorText,
	)
	testutil.CheckEqual(t, strings.Contains(err.Error(), errorText), true,
		"error=%v", err,
	)
}

func TestUnusedCommandCreatesNoDraft(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TMPDIR", dir)
	t.Setenv("EDITOR", "true")
	cmd, err := EditComment(
		context.Background(),
		"body",
		comments.Anchor{},
		func(string, error) tea.Msg { return nil },
	)
	testutil.RequireEqual(t, err, nil, "cmd=%v err=%v", cmd, err)
	testutil.RequireEqual(t, cmd != nil, true, "cmd=%v err=%v", cmd, err)

	_ = cmd() // Producing Bubble Tea's execution message must also be lazy.
	files, err := os.ReadDir(dir)
	testutil.RequireEqual(t, err, nil, "files=%v err=%v", files, err)
	testutil.RequireEqual(t, len(files), 0, "files=%v err=%v", files, err)
}

func TestCommentCancellationAndCreationFailure(t *testing.T) {
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
			adapter, complete := newCommentEditorWorkflow(
				ctx,
				"true",
				"body",
				comments.Anchor{},
				func(
					body string,
					err error,
				) tea.Msg {
					testutil.RequireEqual(t, err != nil, true, "body=%q err=%v",
						body, err,
					)
					testutil.RequireEqual(t, body, "", "body=%q err=%v", body,
						err,
					)
					return nil
				},
			)
			complete(adapter.Run())
			files, err := os.ReadDir(dir)
			testutil.NoError(t, err)
			testutil.RequireEqual(t, len(files), 0, "files=%v", files)
		})
	}
}

func TestTerminalFailureReachesCompletion(t *testing.T) {
	sentinel := errors.New("terminal restoration failed")
	adapter, complete := newCommentEditorWorkflow(
		context.Background(),
		"true",
		"body",
		comments.Anchor{},
		func(
			body string,
			err error,
		) tea.Msg {
			testutil.RequireEqual(t, body, "body", "body=%q err=%v", body, err)
			testutil.RequireEqual(t, errors.Is(err, sentinel), true,
				"body=%q err=%v", body, err,
			)

			return nil
		},
	)
	testutil.NoError(t, adapter.Run())

	complete(sentinel)
}

func TestMissingEditor(t *testing.T) {
	t.Setenv("EDITOR", " \t")
	cmd, err := EditComment(context.Background(), "", comments.Anchor{}, nil)
	testutil.RequireEqual(t, cmd == nil, true, "cmd=%v err=%v", cmd, err)
	testutil.RequireEqual(t, err != nil, true, "cmd=%v err=%v", cmd, err)
	cmd, err = OpenSource(context.Background(), "source", 2, nil)
	testutil.RequireEqual(t, cmd == nil, true, "cmd=%v err=%v", cmd, err)
	testutil.RequireEqual(t, err != nil, true, "cmd=%v err=%v", cmd, err)
}

func TestSourceArgumentsAndContext(t *testing.T) {
	var output bytes.Buffer
	cmd := newSourceEditorCommand(
		context.Background(),
		"printf '%s\\n'",
		"/repo with spaces/it's.go",
		42,
	)
	cmd.Stdout = &output
	testutil.NoError(t, cmd.Run())

	if got := output.String(); got != "+42\n/repo with spaces/it's.go\n" {
		t.Fatalf("arguments = %q", got)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := newSourceEditorCommand(ctx, "true", "source", 2).Run()
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v", err)
	}
}

type completionMsg struct {
	body string
	err  error
}
type workflowModel struct{ command tea.Cmd }

func (m workflowModel) Init() tea.Cmd { return m.command }
func (m workflowModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if _, ok := msg.(completionMsg); ok {
		return m, tea.Quit
	}
	return m, nil
}
func (workflowModel) View() tea.View { return tea.NewView("") }

func TestEditCommentThroughBubbleTea(t *testing.T) {
	t.Setenv("EDITOR", "printf edited >")
	var result completionMsg
	called := false
	cmd, err := EditComment(
		context.Background(),
		"original",
		comments.Anchor{},
		func(
			body string,
			err error,
		) tea.Msg {
			result = completionMsg{body, err}
			called = true
			return result
		},
	)
	testutil.NoError(t, err)

	program := tea.NewProgram(
		workflowModel{cmd},
		tea.WithInput(nil),
		tea.WithOutput(&bytes.Buffer{}),
		tea.WithoutRenderer(),
	)
	if _, err := program.Run(); err != nil {
		t.Fatal(err)
	}
	testutil.RequireEqual(t, called, true, "called=%v result=%#v", called,
		result,
	)
	testutil.RequireEqual(t, result.body, "edited", "called=%v result=%#v",
		called, result,
	)
	testutil.RequireEqual(t, result.err, nil, "called=%v result=%#v", called,
		result,
	)
}

type cancelOnWrite struct{ cancel context.CancelFunc }

func (w cancelOnWrite) Write(p []byte) (int, error) {
	w.cancel()
	return len(p), nil
}

func TestRunningEditorCancellationCleansDraft(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TMPDIR", dir)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	adapter, complete := newCommentEditorWorkflow(
		ctx,
		"printf started; exec sleep 30 #",
		"body",
		comments.Anchor{},
		func(
			body string,
			err error,
		) tea.Msg {
			testutil.RequireEqual(t, err != nil, true, "body=%q error=%v", body,
				err,
			)
			testutil.RequireEqual(t, body, "", "body=%q error=%v", body, err)

			return nil
		},
	)
	adapter.SetStdout(cancelOnWrite{cancel})
	complete(adapter.Run())
	files, err := os.ReadDir(dir)
	testutil.RequireEqual(t, err, nil, "files=%v error=%v", files, err)
	testutil.RequireEqual(t, len(files), 0, "files=%v error=%v", files, err)
}
