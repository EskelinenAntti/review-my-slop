package diffscreen

import (
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

func TestPreserveTranslatesCursorSelectionAndViewport(t *testing.T) {
	old := unifiedProjection(testPatch(), true)
	first := mustFirst(t, old)
	cursor, ok := old.findMatch("added one", first, Forward)
	if !ok {
		t.Fatal("cursor not found")
	}
	selection := diffSelection{First: first, Last: first}
	selection, ok = old.extendSelection(selection, cursor)
	if !ok {
		t.Fatal("selection not created")
	}
	viewport := old.newViewport(100, 4)
	viewport = old.align(viewport, cursor, Center)
	screen := &View{projection: old, cursor: cursor, selection: &selection, viewport: viewport}

	next := splitProjection(testPatch(), true)
	screen.replaceProjection(next)
	preserved := screen

	if !next.hasSourceLine(preserved.cursor) {
		t.Fatal("cursor was not preserved")
	}
	source := next.sourceAt(preserved.cursor)
	line, ok := source.line, source.valid
	if !ok || line.Text != "added one" {
		t.Fatalf("cursor line = %#v, ok=%v", line, ok)
	}
	if preserved.selection == nil {
		t.Fatalf("selection = %#v", preserved.selection)
	}
	firstSource := next.sourceAt(preserved.selection.First)
	lastSource := next.sourceAt(preserved.selection.Last)
	firstLine, lastLine := firstSource.line, lastSource.line
	if !firstSource.valid || !lastSource.valid || firstLine.Text != "before" || lastLine.Text != "added one" {
		t.Fatalf("selection endpoints = %#v, %#v", firstLine, lastLine)
	}
	if got, want := preserved.cursor.row-preserved.viewport.top, cursor.row-viewport.top; got != want {
		t.Fatalf("screen row = %d, want %d", got, want)
	}
}

func TestPreserveReturnsEmptyStateForEmptyView(t *testing.T) {
	old := unifiedProjection(testPatch(), true)
	cursor := mustFirst(t, old)
	screen := &View{projection: old, cursor: cursor, selection: ptr(diffSelection{First: cursor, Last: cursor}), viewport: old.newViewport(80, 10)}

	screen.replaceProjection(unifiedProjection(patch.Patch{}, true))
	preserved := screen

	if preserved.projection.hasSourceLine(preserved.cursor) || preserved.selection != nil {
		t.Fatalf("state = %#v", preserved)
	}
}

func TestPreserveClampsHorizontalOffsetForShorterLines(t *testing.T) {
	p := longPatch()
	p.Files[0].OldPath, p.Files[0].NewPath = "long.go", "long.go"
	old := unifiedProjection(p, true)
	cursor := mustFirst(t, old)
	viewport := old.scrollHorizontal(old.newViewport(40, 10), 30)
	if viewport.LeftColumn == 0 {
		t.Fatal("fixture does not scroll horizontally")
	}
	short := testPatch()
	preserved := &View{projection: old, cursor: cursor, viewport: viewport}
	preserved.replaceProjection(unifiedProjection(short, true))
	if preserved.viewport.LeftColumn != 0 {
		t.Fatalf("horizontal offset = %d", preserved.viewport.LeftColumn)
	}
}

func ptr[T any](value T) *T { return &value }
