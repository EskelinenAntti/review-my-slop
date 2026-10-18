package app

import (
	"context"
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/eskelinenantti/review-my-slop/internal/comments"
	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
	"github.com/eskelinenantti/review-my-slop/internal/ui/diffscreen"
)

func testModel(
	p patch.Patch,
	items []comments.Comment,
	save saveCommentFunc,
) model {
	return newModel(
		context.Background(),
		p,
		items,
		save,
		initialLayout{dimensions: terminalDimensions{width: 100, height: 30}},
	)
}

func TestViewSwitchPreservesSemanticCursor(t *testing.T) {
	m := testModel(modelPatch(), nil, nil)
	m = updateModel(t, m, tea.WindowSizeMsg{Width: 120, Height: 30})
	m.diffView.Move(diffscreen.NextLine)
	m.diffView.Move(diffscreen.NextLine)
	_, want, _ := m.diffView.Current()
	m.setSideBySide(true)
	_, got, ok := m.diffView.Current()
	testutil.RequireEqual(t, ok, true, "focused line=%#v, want %#v", got, want)
	testutil.RequireEqual(t, got, want, "focused line=%#v, want %#v", got, want)
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

func modelPatch() patch.Patch {
	return patch.Patch{
		Root: "/repo",
		Files: []patch.File{
			{
				DisplayPath: "main.go",
				OldPath:     "main.go",
				NewPath:     "main.go",
				Hunks: []patch.Hunk{
					{
						Header: "@@ -1,2 +1,2 @@",
						Lines: []patch.Line{
							{
								Kind:      patch.Context,
								Text:      "keep()",
								OldNumber: 1,
								NewNumber: 1,
							},
							{Kind: patch.Deletion, Text: "old()", OldNumber: 2},
							{Kind: patch.Addition, Text: "new()", NewNumber: 2},
						},
					},
				},
			},
		},
	}
}
