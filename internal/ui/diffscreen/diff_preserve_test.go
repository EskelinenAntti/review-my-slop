package diffscreen

import (
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

func TestPreserveTranslatesCursorSelectionAndViewport(t *testing.T) {
	old := newUnifiedView(testPatch(), true)
	first := mustFirst(t, old)
	cursor, ok := old.search("added one", first, Forward)
	if !ok {
		t.Fatal("cursor not found")
	}
	selection := old.beginSelection(first)
	selection, ok = old.extendSelection(selection, cursor)
	if !ok {
		t.Fatal("selection not created")
	}
	viewport := old.newViewport(100, 4)
	viewport = old.align(viewport, cursor, Center)
	screen := &View{view: old, cursor: cursor, selection: &selection, viewport: viewport}

	next := newSideBySideView(testPatch(), true)
	screen.replaceView(next)
	preserved := screen

	if !next.valid(preserved.cursor) {
		t.Fatal("cursor was not preserved")
	}
	line, ok := next.line(preserved.cursor)
	if !ok || line.Text != "added one" {
		t.Fatalf("cursor line = %#v, ok=%v", line, ok)
	}
	if preserved.selection == nil {
		t.Fatalf("selection = %#v", preserved.selection)
	}
	firstLine, firstOK := next.line(preserved.selection.First)
	lastLine, lastOK := next.line(preserved.selection.Last)
	if !firstOK || !lastOK || firstLine.Text != "before" || lastLine.Text != "added one" {
		t.Fatalf("selection endpoints = %#v, %#v", firstLine, lastLine)
	}
	if got, want := preserved.cursor.row-preserved.viewport.top, cursor.row-viewport.top; got != want {
		t.Fatalf("screen row = %d, want %d", got, want)
	}
}

func TestPreserveReturnsEmptyStateForEmptyView(t *testing.T) {
	old := newUnifiedView(testPatch(), true)
	cursor := mustFirst(t, old)
	screen := &View{view: old, cursor: cursor, selection: ptr(old.beginSelection(cursor)), viewport: old.newViewport(80, 10)}

	screen.replaceView(newUnifiedView(patch.Patch{}, true))
	preserved := screen

	if preserved.view.valid(preserved.cursor) || preserved.selection != nil {
		t.Fatalf("state = %#v", preserved)
	}
}

func TestPreserveClampsHorizontalOffsetForShorterLines(t *testing.T) {
	p := longPatch()
	p.Files[0].OldPath, p.Files[0].NewPath = "long.go", "long.go"
	old := newUnifiedView(p, true)
	cursor := mustFirst(t, old)
	viewport := old.scrollHorizontal(old.newViewport(40, 10), 30)
	if viewport.LeftColumn == 0 {
		t.Fatal("fixture does not scroll horizontally")
	}
	short := testPatch()
	preserved := &View{view: old, cursor: cursor, viewport: viewport}
	preserved.replaceView(newUnifiedView(short, true))
	if preserved.viewport.LeftColumn != 0 {
		t.Fatalf("horizontal offset = %d", preserved.viewport.LeftColumn)
	}
}

func ptr[T any](value T) *T { return &value }
