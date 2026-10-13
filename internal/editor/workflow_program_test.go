package editor

import (
	"bytes"
	"context"
	"fmt"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/eskelinenantti/review-my-slop/internal/comments"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

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
	testutil.RequireEqual(t, called, true,
		fmt.Sprintf("called=%v result=%#v", called, result),
	)
	testutil.RequireEqual(t, result.body, "edited",
		fmt.Sprintf("called=%v result=%#v", called, result),
	)
	testutil.RequireEqual(t, result.err, nil,
		fmt.Sprintf("called=%v result=%#v", called, result),
	)
}
