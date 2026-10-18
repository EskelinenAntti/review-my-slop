package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/eskelinenantti/review-my-slop/internal/comments"
	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestWorkflowErrorKeepsDiffLabelWithinFooter(t *testing.T) {
	for _, width := range []int{20, 80} {
		for _, kind := range []patch.Kind{patch.Unstaged, patch.Branch} {
			p := modelPatch()
			p.Kind, p.Branch = kind, "main"
			m := newModel(
				context.Background(),
				p,
				nil,
				nil,
				initialLayout{
					dimensions: terminalDimensions{width: width, height: 8},
				},
			)
			m.err = errors.New(
				"a very long storage failure\n" +
					"that exceeds the available footer width",
			)
			lines := strings.Split(m.render(), "\n")
			testutil.RequireEqual(t, len(lines), 8,
				"error changed screen height: %d", len(lines),
			)

			footer := lines[len(lines)-2]
			label := "local changes"
			if kind == patch.Branch {
				label = "branch changes from main"
			}
			testutil.RequireEqual(t,
				strings.HasSuffix(ansi.Strip(footer), label), true,
				"footer lost label: %q", footer,
			)
			testutil.RequireEqual(

				// Labels longer than the terminal retain
				// the existing rendering behavior.
				t,
				width < len(label)+1 || lipgloss.Width(footer) == width, true,
				"footer width=%d want=%d", lipgloss.Width(footer), width,
			)
		}
	}
}

func TestRenderingAndKeyBindingsRemainAvailable(t *testing.T) {
	m := testModel(modelPatch(), nil, nil)
	m.dimensions = terminalDimensions{width: 80, height: 10}
	m.resizeScreens()
	m.resizeScreens()
	rendered := m.render()
	for _, value := range []string{
		"review-my-slop",
		"+1-1",
		"old()",
		"new()",
		"local changes",
	} {
		testutil.RequireEqual(t, strings.Contains(rendered, value), true,
			"render missing %q: %q", value, rendered,
		)
	}
	m.mode = modeHelp
	testutil.RequireEqual(t, strings.Contains(m.render(), "Ctrl-w h/l/w"), true,
		"%s", "help lost patch pane binding",
	)
}

func TestEmptyViewKeepsKeyboardHintAtBottom(t *testing.T) {
	m := testModel(patch.Patch{}, nil, nil)
	m.dimensions = terminalDimensions{width: 80, height: 10}
	m.resizeScreens()

	lines := strings.Split(m.render(), "\n")
	got, want := lines[m.dimensions.height-2], "j/k/h/l move"
	if !strings.Contains(got, want) {
		t.Fatalf(
			"line %d = %q, want it to contain %q",
			m.dimensions.height-1,
			got,
			want,
		)
	}
	if got := lines[2]; !strings.Contains(
		got,
		"No unstaged or untracked changes.",
	) {
		t.Fatalf("empty-viewState line = %q", got)
	}
}

func TestMenuKeyboardHintsStayAtBottom(t *testing.T) {
	m := testModel(
		modelPatch(),
		[]comments.Comment{
			{
				Body:   "first",
				Anchor: comments.Anchor{FilePath: "main.go", NewStart: 2},
			},
		},
		nil,
	)
	m.dimensions = terminalDimensions{width: 80, height: 10}
	m.resizeScreens()

	tests := []struct {
		name string
		mode mode
		hint string
	}{
		{name: "comments", mode: modeComments, hint: "j/k move"},
		{name: "help", mode: modeHelp, hint: "? or Esc closes help"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			m.mode = test.mode
			lines := strings.Split(m.render(), "\n")
			if got := lines[m.dimensions.height-2]; !strings.Contains(
				got,
				test.hint,
			) {
				t.Fatalf(
					"line %d = %q, want it to contain %q",
					m.dimensions.height-1,
					got,
					test.hint,
				)
			}
		})
	}
}

func TestCommentsMenuScrollsWithinScreenBody(t *testing.T) {
	items := make([]comments.Comment, 10)
	for index := range items {
		items[index] = comments.Comment{
			Body:   fmt.Sprintf("comment %d", index),
			Anchor: comments.Anchor{FilePath: "main.go"},
		}
	}
	m := testModel(modelPatch(), items, nil)
	m.dimensions = terminalDimensions{width: 80, height: 7}
	m.mode = modeComments
	m.resizeScreens()
	m.commentView.Move(len(items))

	rendered := strings.Split(ansi.Strip(m.render()), "\n")
	testutil.RequireEqual(t,
		strings.Contains(
			strings.Join(rendered[1:m.dimensions.height-2], "\n"),
			"comment 9",
		),
		true, "selected comment is outside the screen body: %q", rendered,
	)
	testutil.RequireEqual(
		t,
		strings.Contains(rendered[m.dimensions.height-2], "j/k move"),
		true, "footer line = %q", rendered[m.dimensions.height-2],
	)
}
