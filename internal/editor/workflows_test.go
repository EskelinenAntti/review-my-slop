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
			if err := os.WriteFile(script, []byte("printf '%s' \"$1\" > "+shellQuote(record)+"\n"+tc.script+"\n"), 0600); err != nil {
				t.Fatal(err)
			}
			adapter, complete := commentWorkflow(context.Background(), "sh "+shellQuote(script), tc.body, comments.Anchor{QuotedLines: []string{" context", "+added"}}, func(body string, err error) tea.Msg {
				if body != tc.want {
					t.Errorf("body = %q, want %q", body, tc.want)
				}
				if tc.errorText == "" && err != nil {
					t.Errorf("error = %v", err)
				}
				if tc.errorText != "" && (err == nil || !strings.Contains(err.Error(), tc.errorText)) {
					t.Errorf("error = %v", err)
				}
				return nil
			})
			complete(adapter.Run())
			path, err := os.ReadFile(record)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(string(path)); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("draft still exists: %v", err)
			}
		})
	}
}

func TestUnusedCommandCreatesNoDraft(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TMPDIR", dir)
	t.Setenv("EDITOR", "true")
	cmd, err := EditComment(context.Background(), "body", comments.Anchor{}, func(string, error) tea.Msg { return nil })
	if err != nil || cmd == nil {
		t.Fatalf("cmd=%v err=%v", cmd, err)
	}
	_ = cmd() // Producing Bubble Tea's execution message must also be lazy.
	files, err := os.ReadDir(dir)
	if err != nil || len(files) != 0 {
		t.Fatalf("files=%v err=%v", files, err)
	}
}

func TestCommentCancellationAndCreationFailure(t *testing.T) {
	for _, canceled := range []bool{false, true} {
		t.Run(map[bool]string{false: "creation failure", true: "cancellation"}[canceled], func(t *testing.T) {
			dir := t.TempDir()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if canceled {
				cancel()
				t.Setenv("TMPDIR", dir)
			} else {
				t.Setenv("TMPDIR", filepath.Join(dir, "missing"))
			}
			adapter, complete := commentWorkflow(ctx, "true", "body", comments.Anchor{}, func(body string, err error) tea.Msg {
				if err == nil || body != "" {
					t.Fatalf("body=%q err=%v", body, err)
				}
				return nil
			})
			complete(adapter.Run())
			files, err := os.ReadDir(dir)
			if err != nil || len(files) != 0 {
				t.Fatalf("files=%v err=%v", files, err)
			}
		})
	}
}

func TestTerminalFailureReachesCompletion(t *testing.T) {
	sentinel := errors.New("terminal restoration failed")
	adapter, complete := commentWorkflow(context.Background(), "true", "body", comments.Anchor{}, func(body string, err error) tea.Msg {
		if body != "body" || !errors.Is(err, sentinel) {
			t.Fatalf("body=%q err=%v", body, err)
		}
		return nil
	})
	if err := adapter.Run(); err != nil {
		t.Fatal(err)
	}
	complete(sentinel)
}

func TestMissingEditor(t *testing.T) {
	t.Setenv("EDITOR", " \t")
	if cmd, err := EditComment(context.Background(), "", comments.Anchor{}, nil); cmd != nil || err == nil {
		t.Fatalf("cmd=%v err=%v", cmd, err)
	}
	if cmd, err := OpenSource(context.Background(), "source", 2, nil); cmd != nil || err == nil {
		t.Fatalf("cmd=%v err=%v", cmd, err)
	}
}

func TestSourceArgumentsAndContext(t *testing.T) {
	var output bytes.Buffer
	cmd := sourceCommand(context.Background(), "printf '%s\\n'", "/repo with spaces/it's.go", 42)
	cmd.Stdout = &output
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	if got := output.String(); got != "+42\n/repo with spaces/it's.go\n" {
		t.Fatalf("arguments = %q", got)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := sourceCommand(ctx, "true", "source", 2).Run(); !errors.Is(err, context.Canceled) {
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
	if err != nil {
		t.Fatal(err)
	}
	program := tea.NewProgram(workflowModel{cmd}, tea.WithInput(nil), tea.WithOutput(&bytes.Buffer{}), tea.WithoutRenderer())
	if _, err := program.Run(); err != nil {
		t.Fatal(err)
	}
	if !called || result.body != "edited" || result.err != nil {
		t.Fatalf("called=%v result=%#v", called, result)
	}
}

type cancelOnWrite struct{ cancel context.CancelFunc }

func (w cancelOnWrite) Write(p []byte) (int, error) { w.cancel(); return len(p), nil }

func TestRunningEditorCancellationCleansDraft(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TMPDIR", dir)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	adapter, complete := commentWorkflow(ctx, "printf started; exec sleep 30 #", "body", comments.Anchor{}, func(body string, err error) tea.Msg {
		if err == nil || body != "" {
			t.Fatalf("body=%q error=%v", body, err)
		}
		return nil
	})
	adapter.SetStdout(cancelOnWrite{cancel})
	complete(adapter.Run())
	files, err := os.ReadDir(dir)
	if err != nil || len(files) != 0 {
		t.Fatalf("files=%v error=%v", files, err)
	}
}
