package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/eskelinenantti/review-my-slop/internal/patch"
	commentscreen "github.com/eskelinenantti/review-my-slop/internal/ui/comments"
	patchscreen "github.com/eskelinenantti/review-my-slop/internal/ui/patch"
)

func TestPatchCommentEventsRouteAcrossScreens(t *testing.T) {
	t.Setenv("EDITOR", "")
	model := New(Initial{
		Patch: patch.Patch{Files: []patch.File{{
			DisplayPath: "main.go",
			NewPath:     "main.go",
			Hunks:       []patch.Hunk{{Lines: []patch.Line{{Kind: patch.Addition, Text: "new()", NewNumber: 1}}}},
		}}},
		Size: Size{Width: 80, Height: 12},
	}, Dependencies{})
	model.Update(keyPress("v"))
	if !strings.Contains(model.Render(), "visual selection") {
		t.Fatal("Patch selection did not start")
	}

	_, request := model.Update(keyPress("c"))
	if request == nil {
		t.Fatal("Patch c key did not request a Comment")
	}
	requestMessage := request()
	requested, ok := requestMessage.(patchscreen.CommentRequested)
	if !ok {
		t.Fatalf("Patch c result = %T, want CommentRequested", requestMessage)
	}
	if requested.Anchor.FilePath != "main.go" || requested.Anchor.NewStart != 1 {
		t.Fatalf("Patch selection anchor = %#v", requested.Anchor)
	}
	_, begin := model.Update(requested)
	if begin == nil {
		t.Fatal("Patch comment request did not start the Comment screen workflow")
	}
	result := begin()
	failed, ok := result.(commentscreen.Failed)
	if !ok {
		t.Fatalf("Begin result = %T, want comments.Failed", result)
	}
	model.Update(failed)
	if !strings.Contains(model.Render(), "$EDITOR is not set") {
		t.Fatalf("Patch-originated failure was not shown on the Patch screen:\n%s", model.Render())
	}

	model.Update(commentscreen.Saved{FromPatch: true})
	if strings.Contains(model.Render(), "visual selection") {
		t.Fatal("successful Patch-originated save did not cancel Patch selection")
	}

	model.Update(keyPress("v"))
	model.Update(commentscreen.Cancelled{FromPatch: true})
	if strings.Contains(model.Render(), "visual selection") {
		t.Fatal("discarded Patch-originated draft did not cancel Patch selection")
	}
}

func keyPress(text string) tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Text: text, Code: rune(text[0])})
}
