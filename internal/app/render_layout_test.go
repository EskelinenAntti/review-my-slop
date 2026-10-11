package app

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/eskelinenantti/review-my-slop/internal/comments"
	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

func TestRenderingAndKeyBindingsRemainAvailable(t *testing.T) {
	m := testModel(modelPatch(), nil, nil)
	m.width, m.height = defaultWidth, 10
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
		requireEqual(
			t,
			strings.Contains(rendered, value), true,
			"render missing %q: %q",
			value,
			rendered,
		)
	}
	m.mode = modeHelp
	requireEqual(
		t,
		strings.Contains(m.render(), "Ctrl-w h/l/w"),
		true,
		valueFormat,
		"help lost diffPane binding",
	)
}

func TestEmptyViewKeepsKeyboardHintAtBottom(t *testing.T) {
	m := testModel(patch.Patch{}, nil, nil)
	m.width, m.height = defaultWidth, 10
	m.resizeScreens()

	lines := strings.Split(m.render(), newline)
	{
		got, want := lines[m.height-2], "j/k/h/l move"
		requireEqual(
			t,
			strings.Contains(got, want),
			true,
			"line %d = %q, want it to contain %q",
			m.height-1,
			got,
			want,
		)
	}
	{
		got := lines[2]
		requireEqual(
			t,
			strings.Contains(got, "No unstaged or untracked changes."),
			true,
			"empty-viewState line = %q",
			got,
		)
	}
}

func TestMenuKeyboardHintsStayAtBottom(t *testing.T) {
	m := testModel(
		modelPatch(),
		[]comments.Comment{
			{
				Body: "first",
				Anchor: comments.Anchor{
					FilePath: fixtureMainGo,
					NewStart: 2,
				},
			},
		},
		nil,
	)
	m.width, m.height = defaultWidth, 10
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
		t.Run(
			test.name,
			func(t *testing.T) {
				m.mode = test.mode
				lines := strings.Split(m.render(), newline)
				{
					got := lines[m.height-2]
					requireEqual(
						t,
						strings.Contains(got, test.hint),
						true,
						"line %d = %q, want it to contain %q",
						m.height-1,
						got,
						test.hint,
					)
				}
			},
		)
	}
}

func TestCommentsMenuScrollsWithinScreenBody(t *testing.T) {
	items := make([]comments.Comment, 10)
	for index := range items {
		items[index] = comments.Comment{
			Body:   fmt.Sprintf("comment %d", index),
			Anchor: comments.Anchor{FilePath: fixtureMainGo},
		}
	}
	m := testModel(modelPatch(), items, nil)
	m.width, m.height = defaultWidth, 7
	m.mode = modeComments
	m.resizeScreens()
	m.commentView.Move(len(items))

	rendered := strings.Split(ansi.Strip(m.render()), newline)
	requireEqual(
		t,
		strings.Contains(
			strings.Join(rendered[1:m.height-2], newline),
			"comment 9",
		),
		true,
		"selected comment is outside the screen body: %q",
		rendered,
	)
	requireEqual(
		t,
		strings.Contains(rendered[m.height-2], "j/k move"), true,
		"footer line = %q",
		rendered[m.height-2],
	)
}
