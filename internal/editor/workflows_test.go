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
)

func TestCommentWorkflowLifecycle(t *testing.T) {
	for _, tc := range []workflowCase{
		{"edited", `printf edited > "$1"`, fixtureOriginal, "edited", ""},

		{
			"unchanged suggestion",
			`true`,
			fixtureOriginal,
			fixtureOriginal,
			"",
		},

		{"failed process", `exit 7`, fixtureOriginal, "", "editor:"},

		{"failed read", `rm "$1"`, fixtureOriginal, "", "read comment file:"},
	} {
		t.Run(
			tc.name,
			func(t *testing.T) {
				assertCommentWorkflow(t, tc)
			},
		)
	}
}

func TestUnusedCommandCreatesNoDraft(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(fixtureTMPDIR, dir)
	t.Setenv(editorVariable, fixtureTrue)
	cmd, err := EditComment(
		context.Background(),
		fixtureBody,
		comments.Anchor{},
		func(
			string,
			error,
		) tea.Msg {
			return nil
		},
	)
	requireEqual(
		t,
		err == nil && cmd != nil, true,
		commandErrorFormat,
		cmd,
		err,
	)
	_ = cmd() // Producing Bubble Tea's execution message must also be lazy.
	files, err := os.ReadDir(dir)
	requireEqual(
		t,
		err == nil && len(files) == 0, true,
		"files=%v err=%v",
		files,
		err,
	)
}

func TestCommentCancellationAndCreationFailure(t *testing.T) {
	for _, canceled := range []bool{false, true} {
		t.Run(

			map[bool]string{
				false: "creation failure",
				true:  "cancellation",
			}[canceled],

			func(
				t *testing.T,
			) {
				assertWorkflowFailure(
					t,
					map[bool]string{
						false: "creation failure",
						true:  "cancellation",
					}[canceled],
				)
			},
		)
	}
}

func TestTerminalFailureReachesCompletion(t *testing.T) {
	sentinel := errors.New("terminal restoration failed")
	adapter, complete := commentWorkflow(
		context.Background(),
		fixtureTrue,
		fixtureBody,
		comments.Anchor{},
		func(
			body string,
			err error,
		) tea.Msg {
			requireEqual(
				t,
				body == fixtureBody && errors.Is(err, sentinel), true,
				"body=%q err=%v",
				body,
				err,
			)
			return nil
		},
	)
	{
		err := adapter.Run()
		requireEqual(t, err == nil, true, fixtureV, err)
	}
	complete(sentinel)
}

func TestMissingEditor(t *testing.T) {
	t.Setenv(editorVariable, " \t")
	{
		cmd, err := EditComment(
			context.Background(),
			"",
			comments.Anchor{},
			nil,
		)
		requireEqual(
			t,
			cmd == nil && err != nil,
			true,
			commandErrorFormat,
			cmd,
			err,
		)
	}
	{
		cmd, err := OpenSource(context.Background(), "source", two, nil)
		requireEqual(
			t,
			cmd == nil && err != nil,
			true,
			commandErrorFormat,
			cmd,
			err,
		)
	}
}

func TestSourceArgumentsAndContext(t *testing.T) {
	var output bytes.Buffer
	cmd := sourceCommand(
		context.Background(),
		"printf '%s\\n'",
		"/repo with spaces/it's.go",
		sourceLine,
	)
	cmd.Stdout = &output
	{
		err := cmd.Run()
		requireEqual(t, err == nil, true, fixtureV, err)
	}
	{
		got := output.String()
		requireEqual(
			t,
			got, "+42\n/repo with spaces/it's.go\n",
			"arguments = %q",
			got,
		)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	{
		err := sourceCommand(ctx, fixtureTrue, "source", two).Run()
		requireEqual(
			t,
			errors.Is(err, context.Canceled),
			true,
			"error = %v",
			err,
		)
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
	t.Setenv(editorVariable, "printf edited >")
	var result completionMsg
	called := false
	cmd, err := EditComment(
		context.Background(),
		fixtureOriginal,
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
	requireEqual(t, err == nil, true, fixtureV, err)
	program := tea.NewProgram(
		workflowModel{cmd},
		tea.WithInput(nil),
		tea.WithOutput(&bytes.Buffer{}),
		tea.WithoutRenderer(),
	)
	{
		_, err := program.Run()
		requireEqual(t, err == nil, true, fixtureV, err)
	}
	requireEqual(
		t,
		called && result.body == "edited" && result.err == nil, true,
		"called=%v result=%#v",
		called,
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
	t.Setenv(fixtureTMPDIR, dir)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	adapter, complete := commentWorkflow(
		ctx,
		"printf started; exec sleep 30 #",
		fixtureBody,
		comments.Anchor{},
		func(
			body string,
			err error,
		) tea.Msg {
			requireEqual(
				t,
				err != nil && body == "", true,
				"body=%q error=%v",
				body,
				err,
			)
			return nil
		},
	)
	adapter.SetStdout(cancelOnWrite{cancel})
	complete(adapter.Run())
	files, err := os.ReadDir(dir)
	requireEqual(
		t,
		err == nil && len(files) == 0, true,
		"files=%v error=%v",
		files,
		err,
	)
}

func assertWorkflowFailure(t *testing.T, failure string) {
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if failure == "cancellation" {
		cancel()
		t.Setenv(fixtureTMPDIR, dir)
	} else {
		t.Setenv(fixtureTMPDIR, filepath.Join(dir, "missing"))
	}
	adapter, complete := commentWorkflow(
		ctx,
		fixtureTrue,
		fixtureBody,
		comments.Anchor{},
		func(
			body string,
			err error,
		) tea.Msg {
			requireEqual(
				t,
				err != nil && body == "", true,
				"body=%q err=%v",
				body,
				err,
			)
			return nil
		},
	)
	complete(adapter.Run())
	files, err := os.ReadDir(dir)
	requireEqual(
		t,
		err == nil && len(files) == 0, true,
		"files=%v err=%v",
		files,
		err,
	)
}

type workflowCase struct{ name, script, body, want, errorText string }

func assertCommentWorkflow(t *testing.T, tc workflowCase) {
	dir := t.TempDir()
	record := filepath.Join(dir, "draft-path")
	script := filepath.Join(dir, "editor's script.sh")
	{
		err := os.WriteFile(
			script,
			[]byte(
				"printf '%s' \"$1\" > "+shellQuote(record)+"\n"+
					tc.script+
					"\n",
			),
			privateFileMode,
		)
		requireEqual(t, err == nil, true, fixtureV, err)
	}
	adapter, complete := commentWorkflow(
		context.Background(),
		"sh "+shellQuote(script),
		tc.body,
		comments.Anchor{
			QuotedLines: []string{" context", "+added"},
		},
		func(
			body string,
			err error,
		) tea.Msg {
			checkEqual(
				t,
				body, tc.want,
				"body = %q, want %q",
				body,
				tc.want,
			)
			checkEqual(
				t,
				tc.errorText != "" || err == nil,
				true,
				"error = %v",
				err,
			)
			checkEqual(
				t,
				tc.errorText == "" ||
					err != nil &&
						strings.Contains(
							err.Error(),
							tc.errorText,
						),
				true,
				"error = %v",
				err,
			)
			return nil
		},
	)
	complete(adapter.Run())
	path, err := os.ReadFile(record)
	requireEqual(t, err == nil, true, fixtureV, err)
	{
		_, err := os.Stat(string(path))
		requireEqual(
			t,
			errors.Is(err, os.ErrNotExist), true,
			"draft still exists: %v",
			err,
		)
	}
}
