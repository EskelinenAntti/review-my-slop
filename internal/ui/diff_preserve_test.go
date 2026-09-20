package ui

import (
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

func TestPreserveTranslatesCursorSelectionAndViewport(t *testing.T) {
	old := NewUnifiedView(testPatch(), true)
	first := mustFirst(t, old)
	cursor, ok := old.Search("added one", first, Forward)
	if !ok {
		t.Fatal("cursor not found")
	}
	selection := old.BeginSelection(first)
	selection, ok = old.ExtendSelection(selection, cursor)
	if !ok {
		t.Fatal("selection not created")
	}
	viewport := old.NewViewport(100, 4)
	viewport = old.Align(viewport, cursor, Middle)
	state := State{Cursor: &cursor, Selection: &selection, Viewport: viewport}

	next := NewSideBySideView(testPatch(), true)
	preserved := Preserve(old, state, next)

	if preserved.Cursor == nil {
		t.Fatal("cursor was not preserved")
	}
	line, ok := next.Line(*preserved.Cursor)
	if !ok || line.Text != "added one" {
		t.Fatalf("cursor line = %#v, ok=%v", line, ok)
	}
	if preserved.Selection == nil {
		t.Fatalf("selection = %#v", preserved.Selection)
	}
	firstLine, firstOK := next.Line(preserved.Selection.First)
	lastLine, lastOK := next.Line(preserved.Selection.Last)
	if !firstOK || !lastOK || firstLine.Text != "before" || lastLine.Text != "added one" {
		t.Fatalf("selection endpoints = %#v, %#v", firstLine, lastLine)
	}
	if got, want := preserved.Cursor.Coordinate.Y-preserved.Viewport.Top.Y, cursor.Coordinate.Y-viewport.Top.Y; got != want {
		t.Fatalf("screen row = %d, want %d", got, want)
	}
}

func TestPreserveReturnsEmptyStateForEmptyView(t *testing.T) {
	old := NewUnifiedView(testPatch(), true)
	cursor := mustFirst(t, old)
	state := State{Cursor: &cursor, Selection: ptr(old.BeginSelection(cursor)), Viewport: old.NewViewport(80, 10)}

	preserved := Preserve(old, state, NewUnifiedView(patch.Patch{}, true))

	if preserved.Cursor != nil || preserved.Selection != nil {
		t.Fatalf("state = %#v", preserved)
	}
}

func ptr[T any](value T) *T { return &value }
