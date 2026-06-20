package tui

import (
	"strings"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/review"
)

func TestRefreshTranslatesCursorAndSelection(t *testing.T) {
	m := New(modelPatch(), nil, nil)
	m.move(1)
	selection := m.view.BeginSelection(m.cursor)
	m.selection = &selection
	m.move(1)
	want, _ := m.view.Line(m.cursor)
	refreshed := modelPatch()
	refreshed.Fingerprint = "new"
	refreshed.Files[0].Metadata = []string{"new metadata"}
	m.rebuildView(refreshed)
	got, ok := m.view.Line(m.cursor)
	if !ok || got != want {
		t.Fatalf("cursor line = %#v, want %#v", got, want)
	}
	if m.selection == nil || len(m.view.Lines(*m.selection)) != 2 {
		t.Fatalf("selection was not translated: %#v", m.selection)
	}
}

func TestViewSwitchPreservesSemanticCursor(t *testing.T) {
	m := New(modelPatch(), nil, nil)
	m.width = 120
	m.move(1)
	m.move(1)
	want, _ := m.view.Line(m.cursor)
	oldCoordinate := m.cursor.Coordinate
	m.setSideBySide(true)
	got, ok := m.view.Line(m.cursor)
	if !ok || got != want {
		t.Fatalf("cursor line after switch = %#v", got)
	}
	if m.cursor.Coordinate == oldCoordinate {
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
	m.commentBody = "comment"
	m.editAnchor = review.Anchor{FilePath: "main.go"}
	m.finishCommentEdit()
	if savedPatch.Repository != "/repo" || len(m.comments) != 1 || m.comments[0].Anchor.FilePath != "main.go" {
		t.Fatalf("saved patch/comments = %#v %#v", savedPatch, m.comments)
	}
}

func TestRenderingAndKeyBindingsRemainAvailable(t *testing.T) {
	m := New(modelPatch(), nil, nil)
	m.width, m.height = 80, 10
	m.viewport = m.view.Resize(m.viewport, m.width, m.viewportHeight())
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

func TestCommentDraftRoundTrip(t *testing.T) {
	anchor := review.Anchor{QuotedLines: []string{" old", "-gone", "+new"}}
	draft := commentEditorDraft("body", anchor)
	if got := stripUnchangedSuggestion(draft, anchor.QuotedLines); got != "body" {
		t.Fatalf("unchanged suggestion result = %q", got)
	}
}

func modelPatch() patch.Patch {
	return patch.Patch{Repository: "/repo", Fingerprint: "old", Files: []patch.File{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", Hunks: []patch.Hunk{{Header: "@@ -1,2 +1,2 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "keep()", OldNumber: 1, NewNumber: 1}, {Kind: patch.Deletion, Text: "old()", OldNumber: 2}, {Kind: patch.Addition, Text: "new()", NewNumber: 2}}}}}}}
}
