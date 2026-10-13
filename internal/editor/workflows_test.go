package editor

import (
	"context"
	"errors"
	"fmt"
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
				commentDraftInput{
					body: tc.body,
					anchor: comments.Anchor{
						QuotedLines: []string{" context", "+added"},
					},
				},
				func(body string, err error) tea.Msg {
					assertWorkflowResult(
						t,
						completionMsg{body: body, err: err},
						tc.want,
						tc.errorText,
					)
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
	result completionMsg,
	want, errorText string,
) {
	t.Helper()
	body, err := result.body, result.err
	testutil.CheckEqual(t, body, want,
		fmt.Sprintf("body=%q want=%q", body, want),
	)
	if errorText == "" {
		testutil.NoError(t, err)
		return
	}
	testutil.RequireEqual(t, err != nil, true,
		fmt.Sprintf("expected error containing %q", errorText),
	)
	testutil.CheckEqual(t, strings.Contains(err.Error(), errorText), true,
		fmt.Sprintf("error=%v", err),
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
	testutil.RequireEqual(t, err, nil, fmt.Sprintf("cmd=%v err=%v", cmd, err))
	testutil.RequireEqual(t, cmd != nil, true,
		fmt.Sprintf("cmd=%v err=%v", cmd, err),
	)

	_ = cmd() // Producing Bubble Tea's execution message must also be lazy.
	files, err := os.ReadDir(dir)
	testutil.RequireEqual(t, err, nil,
		fmt.Sprintf("files=%v err=%v", files, err),
	)
	testutil.RequireEqual(t, len(files), 0,
		fmt.Sprintf("files=%v err=%v", files, err),
	)
}

func TestTerminalFailureReachesCompletion(t *testing.T) {
	sentinel := errors.New("terminal restoration failed")
	adapter, complete := newCommentEditorWorkflow(
		context.Background(),
		"true",
		commentDraftInput{body: "body", anchor: comments.Anchor{}},
		func(
			body string,
			err error,
		) tea.Msg {
			testutil.RequireEqual(t, body, "body",
				fmt.Sprintf("body=%q err=%v", body, err),
			)
			testutil.RequireEqual(t, errors.Is(err, sentinel), true,
				fmt.Sprintf("body=%q err=%v", body, err),
			)

			return nil
		},
	)
	testutil.NoError(t, adapter.Run())

	complete(sentinel)
}
