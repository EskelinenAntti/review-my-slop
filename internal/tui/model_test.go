package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/eskelinenantti/review-my-slop/internal/editor"
	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/review"
)

func TestRefreshTranslatesCursorAndSelection(t *testing.T) {
	m := New(modelPatch(), nil, nil)
	m.move(1)
	selection := m.review.view.BeginSelection(m.review.cursor)
	m.review.selection = &selection
	m.move(1)
	want, _ := m.review.view.Line(m.review.cursor)
	refreshed := modelPatch()
	refreshed.Fingerprint = "new"
	refreshed.Files[0].Metadata = []string{"new metadata"}
	m.rebuildView(refreshed)
	got, ok := m.review.view.Line(m.review.cursor)
	if !ok || got != want {
		t.Fatalf("cursor line = %#v, want %#v", got, want)
	}
	if m.review.selection == nil || len(m.review.view.Lines(*m.review.selection)) != 2 {
		t.Fatalf("selection was not translated: %#v", m.review.selection)
	}
}

func TestViewSwitchPreservesSemanticCursor(t *testing.T) {
	m := New(modelPatch(), nil, nil)
	m.width = 120
	m.move(1)
	m.move(1)
	want, _ := m.review.view.Line(m.review.cursor)
	oldCoordinate := m.review.cursor.Coordinate
	m.setSideBySide(true)
	got, ok := m.review.view.Line(m.review.cursor)
	if !ok || got != want {
		t.Fatalf("cursor line after switch = %#v", got)
	}
	if m.review.cursor.Coordinate == oldCoordinate {
		t.Fatal("layout switch reused the old coordinate")
	}
}

func TestCommentSaveUsesPatchAndPreservesAnchor(t *testing.T) {
	var savedPatch patch.Patch
	m := New(modelPatch(), nil, func(stored review.Comment, p patch.Patch) (review.Comment, error) {
		savedPatch = p
		stored.ID = "1"
		return stored, nil
	})
	m.comments.body = "comment"
	m.comments.editAnchor = review.Anchor{FilePath: "main.go"}
	m.finishCommentEdit()
	if savedPatch.Repository != "/repo" || len(m.comments.items) != 1 || m.comments.items[0].Anchor.FilePath != "main.go" {
		t.Fatalf("saved patch/comments = %#v %#v", savedPatch, m.comments.items)
	}
}

func TestRenderingAndKeyBindingsRemainAvailable(t *testing.T) {
	m := New(modelPatch(), nil, nil)
	m.width, m.height = 80, 10
	m.review.viewport = m.review.view.Resize(m.review.viewport, m.width, m.screenBodyHeight())
	rendered := m.render()
	for _, value := range []string{"review-my-slop", "+1-1", "old()", "new()", "local changes"} {
		if !strings.Contains(rendered, value) {
			t.Fatalf("render missing %q: %q", value, rendered)
		}
	}
	m.mode = modeHelp
	if !strings.Contains(m.render(), "Ctrl-w h/l/w") {
		t.Fatal("help lost pane binding")
	}
}

func TestEmptyViewKeepsKeyboardHintAtBottom(t *testing.T) {
	m := New(patch.Patch{}, nil, nil)
	m.width, m.height = 80, 10

	lines := strings.Split(m.render(), "\n")
	if got, want := lines[m.height-2], "j/k/h/l move"; !strings.Contains(got, want) {
		t.Fatalf("line %d = %q, want it to contain %q", m.height-1, got, want)
	}
	if got := lines[2]; !strings.Contains(got, "No unstaged or untracked changes.") {
		t.Fatalf("empty-state line = %q", got)
	}
}

func TestMenuKeyboardHintsStayAtBottom(t *testing.T) {
	m := New(modelPatch(), []review.Comment{{Body: "first", Anchor: review.Anchor{FilePath: "main.go", NewStart: 2}}}, nil)
	m.width, m.height = 80, 10

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
				t.Fatalf("line %d = %q, want it to contain %q", m.height-1, got, test.hint)
			}
		})
	}
}

func TestCommentsMenuScrollsWithinScreenBody(t *testing.T) {
	comments := make([]review.Comment, 10)
	for index := range comments {
		comments[index] = review.Comment{Body: fmt.Sprintf("comment %d", index), Anchor: review.Anchor{FilePath: "main.go"}}
	}
	m := New(modelPatch(), comments, nil)
	m.width, m.height = 80, 7
	m.mode = modeComments
	m.comments.row = len(comments) - 1

	rendered := strings.Split(ansi.Strip(m.render()), "\n")
	if !strings.Contains(strings.Join(rendered[1:m.height-2], "\n"), "comment 9") {
		t.Fatalf("selected comment is outside the screen body: %q", rendered)
	}
	if !strings.Contains(rendered[m.height-2], "j/k move") {
		t.Fatalf("footer line = %q", rendered[m.height-2])
	}
}

func TestCommentDraftRoundTrip(t *testing.T) {
	anchor := review.Anchor{QuotedLines: []string{" old", "-gone", "+new"}}
	draft := editor.CommentDraft("body", anchor)
	if got := editor.StripUnchangedSuggestion(draft, anchor.QuotedLines); got != "body" {
		t.Fatalf("unchanged suggestion result = %q", got)
	}
}

func modelPatch() patch.Patch {
	return patch.Patch{Repository: "/repo", Fingerprint: "old", Files: []patch.File{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", Hunks: []patch.Hunk{{Header: "@@ -1,2 +1,2 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "keep()", OldNumber: 1, NewNumber: 1}, {Kind: patch.Deletion, Text: "old()", OldNumber: 2}, {Kind: patch.Addition, Text: "new()", NewNumber: 2}}}}}}}
}
