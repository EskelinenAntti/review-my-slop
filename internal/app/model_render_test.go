package app

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/eskelinenantti/review-my-slop/internal/comments"
	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestRenderingAndKeyBindingsRemainAvailable(t *testing.T) {
	m := newTestModel(singleHunkPatch(), nil, nil)
	m.width, m.height = 80, 10
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
			fmt.Sprintf("render missing %q: %q", value, rendered),
		)
	}
	m.mode = modeHelp
	testutil.RequireEqual(t, strings.Contains(m.render(), "Ctrl-w h/l/w"),
		true, "help lost diffPane binding",
	)
}

func TestEmptyViewKeepsKeyboardHintAtBottom(t *testing.T) {
	m := newTestModel(patch.Patch{}, nil, nil)
	m.width, m.height = 80, 10
	m.resizeScreens()

	lines := strings.Split(m.render(), "\n")
	if got, want := lines[m.height-2], "j/k/h/l move"; !strings.Contains(
		got,
		want,
	) {
		t.Fatalf("line %d = %q, want it to contain %q", m.height-1, got, want)
	}
	if got := lines[2]; !strings.Contains(
		got,
		"No unstaged or untracked changes.",
	) {
		t.Fatalf("empty-viewState line = %q", got)
	}
}

func TestMenuKeyboardHintsStayAtBottom(t *testing.T) {
	m := newTestModel(
		singleHunkPatch(),
		[]comments.Comment{
			{
				Body:   "first",
				Anchor: comments.Anchor{FilePath: "main.go", NewStart: 2},
			},
		},
		nil,
	)
	m.width, m.height = 80, 10
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
			if got := lines[m.height-2]; !strings.Contains(got, test.hint) {
				t.Fatalf(
					"line %d = %q, want it to contain %q",
					m.height-1,
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
	m := newTestModel(singleHunkPatch(), items, nil)
	m.width, m.height = 80, 7
	m.mode = modeComments
	m.resizeScreens()
	m.commentView.Move(len(items))

	rendered := strings.Split(ansi.Strip(m.render()), "\n")
	testutil.RequireEqual(t, strings.Contains(
		strings.Join(rendered[1:m.height-2], "\n"),
		"comment 9",
	), true,
		fmt.Sprintf("selected comment is outside the screen body: %q",
			rendered,
		),
	)
	testutil.RequireEqual(t,
		strings.Contains(rendered[m.height-2], "j/k move"), true,
		fmt.Sprintf("footer line = %q", rendered[m.height-2]),
	)
}
