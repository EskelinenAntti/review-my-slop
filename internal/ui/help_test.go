package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/eskelinenantti/review-my-slop/internal/comments"
	patchscreen "github.com/eskelinenantti/review-my-slop/internal/ui/patch"
)

func TestRenderKeyBindingsAlignsDescriptions(t *testing.T) {
	lines := renderKeyBindings([]keyBinding{{keys: "x", description: "short"}, {keys: "long", description: "wide"}})
	if strings.Index(lines[0], "short") != strings.Index(lines[1], "wide") {
		t.Fatalf("lines are not aligned: %#v", lines)
	}
}

func TestShellHelpShowsBindingsAndCloses(t *testing.T) {
	model := New(Initial{Size: Size{Width: 80, Height: 12}}, Dependencies{})
	model.Update(patchscreen.HelpRequested{})
	help := model.Render()
	for _, binding := range []string{
		"review-my-slop help", "j/k, arrows", "h/l, left/right", "Ctrl-w h/l/w",
		"0/$", "gg/G", "zz/zt/zb", "Ctrl-d/Ctrl-u", "/",
		"n/N", "]f/[f", "v", "c", "e", "C", "R", "Tab", "t", "q",
	} {
		if !strings.Contains(help, binding) {
			t.Errorf("help omitted %q:\n%s", binding, help)
		}
	}
	lines := strings.Split(help, "\n")
	if strings.TrimSpace(lines[len(lines)-2]) != "? or Esc closes help" {
		t.Fatalf("help footer is not at the bottom: %q", lines[len(lines)-2])
	}
	model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEsc}))
	if strings.Contains(model.Render(), "review-my-slop help") {
		t.Fatal("Escape did not close help")
	}
}

func TestEmptyScreenKeyboardHintsStayAtBottom(t *testing.T) {
	model := New(Initial{Size: Size{Width: 80, Height: 12}}, Dependencies{
		List: func() ([]comments.Comment, error) { return nil, nil },
	})
	patchLines := strings.Split(model.Render(), "\n")
	patchFooter := patchLines[len(patchLines)-2]
	for _, hint := range []string{"j/k/h/l move", "c comment", "? help", "q quit"} {
		if !strings.Contains(patchFooter, hint) {
			t.Fatalf("Patch footer omitted %q: %q", hint, patchFooter)
		}
	}

	model.Update(patchscreen.CommentsRequested{})
	commentLines := strings.Split(model.Render(), "\n")
	if footer := commentLines[len(commentLines)-2]; !strings.Contains(footer, "Esc/q return") {
		t.Fatalf("Comment keyboard hints are not at the bottom: %q", footer)
	}
}
