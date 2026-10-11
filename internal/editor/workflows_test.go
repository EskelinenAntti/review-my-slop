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

			testutil.NoError(t, os.WriteFile(script, []byte("printf '%s' \"$1\" > "+shellQuote(record)+"\n"+tc.script+"\n"), 0600))

			adapter, complete := commentWorkflow(context.Background(), "sh "+shellQuote(script), tc.body, comments.Anchor{QuotedLines: []string{" context", "+added"}}, func(body string, err error) tea.Msg {
				testutil.Assertf(t, body == tc.want, "body = %q, want %q", body, tc.want)
				testutil.Assertf(t, tc.errorText != "" || err == nil, "error = %v", err)
				testutil.Assertf(t, tc.errorText == "" || err != nil && strings.Contains(err.Error(), tc.errorText), "error = %v", err)
				return nil
			})
			complete(adapter.Run())
			path, err := os.ReadFile(record)
			testutil.Require(t, err == nil, err)
			{
				_, err := os.Stat(string(path))
				testutil.Requiref(t, errors.Is(err, os.ErrNotExist), "draft still exists: %v", err)
			}
		})
	}
}

func TestUnusedCommandCreatesNoDraft(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TMPDIR", dir)
	t.Setenv("EDITOR", "true")
	cmd, err := EditComment(context.Background(), "body", comments.Anchor{}, func(string, error) tea.Msg { return nil })
	testutil.Requiref(t, err == nil && cmd != nil, "cmd=%v err=%v", cmd, err)
	_ = cmd() // Producing Bubble Tea's execution message must also be lazy.
	files, err := os.ReadDir(dir)
	testutil.Requiref(t, err == nil && len(files) == 0, "files=%v err=%v", files, err)
}

func TestCommentCancellationAndCreationFailure(t *testing.T) {
	for _, canceled := range []bool{false, true} {
		t.Run(map[bool]string{false: "creation failure", true: "cancellation"}[canceled], func(t *testing.T) {
			dir := t.TempDir()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			configureWorkflowFailure(t, dir, canceled, cancel)
			adapter, complete := commentWorkflow(ctx, "true", "body", comments.Anchor{}, func(body string, err error) tea.Msg {
				testutil.Requiref(t, err != nil && body == "", "body=%q err=%v", body, err)
				return nil
			})
			complete(adapter.Run())
			files, err := os.ReadDir(dir)
			testutil.Requiref(t, err == nil && len(files) == 0, "files=%v err=%v", files, err)
		})
	}
}

func TestTerminalFailureReachesCompletion(t *testing.T) {
	sentinel := errors.New("terminal restoration failed")
	adapter, complete := commentWorkflow(context.Background(), "true", "body", comments.Anchor{}, func(body string, err error) tea.Msg {
		testutil.Requiref(t, body == "body" && errors.Is(err, sentinel), "body=%q err=%v", body, err)
		return nil
	})

	testutil.NoError(t, adapter.Run())

	complete(sentinel)
}

func TestMissingEditor(t *testing.T) {
	t.Setenv("EDITOR", " \t")
	{
		cmd, err := EditComment(context.Background(), "", comments.Anchor{}, nil)
		testutil.Requiref(t, cmd == nil && err != nil, "cmd=%v err=%v", cmd, err)
	}
	{
		cmd, err := OpenSource(context.Background(), "source", 2, nil)
		testutil.Requiref(t, cmd == nil && err != nil, "cmd=%v err=%v", cmd, err)
	}
}

func TestSourceArgumentsAndContext(t *testing.T) {
	var output bytes.Buffer
	cmd := sourceCommand(context.Background(), "printf '%s\\n'", "/repo with spaces/it's.go", 42)
	cmd.Stdout = &output

	testutil.NoError(t, cmd.Run())

	{
		got := output.String()
		testutil.Requiref(t, got == "+42\n/repo with spaces/it's.go\n", "arguments = %q", got)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	{
		err := sourceCommand(ctx, "true", "source", 2).Run()
		testutil.Requiref(t, errors.Is(err, context.Canceled), "error = %v", err)
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
func (m workflowModel) View() tea.View { return tea.NewView("") }

func TestEditCommentThroughBubbleTea(t *testing.T) {
	t.Setenv("EDITOR", "printf edited >")
	var result completionMsg
	called := false
	cmd, err := EditComment(context.Background(), "original", comments.Anchor{}, func(body string, err error) tea.Msg {
		result = completionMsg{body, err}
		called = true
		return result
	})
	testutil.Require(t, err == nil, err)
	program := tea.NewProgram(workflowModel{cmd}, tea.WithInput(nil), tea.WithOutput(&bytes.Buffer{}), tea.WithoutRenderer())
	{
		_, err := program.Run()
		testutil.Require(t, err == nil, err)
	}
	testutil.Requiref(t, called && result.body == "edited" && result.err == nil, "called=%v result=%#v", called, result)
}

type cancelOnWrite struct{ cancel context.CancelFunc }

func (w cancelOnWrite) Write(p []byte) (int, error) { w.cancel(); return len(p), nil }

func TestRunningEditorCancellationCleansDraft(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TMPDIR", dir)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	adapter, complete := commentWorkflow(ctx, "printf started; exec sleep 30 #", "body", comments.Anchor{}, func(body string, err error) tea.Msg {
		testutil.Requiref(t, err != nil && body == "", "body=%q error=%v", body, err)
		return nil
	})
	adapter.SetStdout(cancelOnWrite{cancel})
	complete(adapter.Run())
	files, err := os.ReadDir(dir)
	testutil.Requiref(t, err == nil && len(files) == 0, "files=%v error=%v", files, err)
}

func configureWorkflowFailure(t *testing.T, dir string, canceled bool, cancel context.CancelFunc) {
	t.Helper()
	if canceled {
		cancel()
		t.Setenv("TMPDIR", dir)
	} else {
		t.Setenv("TMPDIR", filepath.Join(dir, "missing"))
	}
}
