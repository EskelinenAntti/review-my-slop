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
	state := State{Selection: &selection, Extending: true, Viewport: viewport}

	next := NewSideBySideView(testPatch(), true)
	preserved := Preserve(old, state, next)

	if preserved.Selection == nil {
		t.Fatal("cursor was not preserved")
	}
	line, ok := next.Line(preserved.Cursor())
	if !ok || line.Text != "added one" {
		t.Fatalf("cursor line = %#v, ok=%v", line, ok)
	}
	if !preserved.Extending {
		t.Fatal("visual extension mode was not preserved")
	}
	firstLine, firstOK := next.Line(preserved.Selection.First)
	lastLine, lastOK := next.Line(preserved.Selection.Last)
	if !firstOK || !lastOK || firstLine.Text != "before" || lastLine.Text != "added one" {
		t.Fatalf("selection endpoints = %#v, %#v", firstLine, lastLine)
	}
	if got, want := preserved.Cursor().Coordinate.Y-preserved.Viewport.Top.Y, cursor.Coordinate.Y-viewport.Top.Y; got != want {
		t.Fatalf("screen row = %d, want %d", got, want)
	}
}

func TestPreserveReturnsEmptyStateForEmptyView(t *testing.T) {
	old := NewUnifiedView(testPatch(), true)
	cursor := mustFirst(t, old)
	state := State{Selection: ptr(old.BeginSelection(cursor)), Extending: true, Viewport: old.NewViewport(80, 10)}

	preserved := Preserve(old, state, NewUnifiedView(patch.Patch{}, true))

	if preserved.Selection != nil || preserved.Extending {
		t.Fatalf("state = %#v", preserved)
	}
}

func ptr[T any](value T) *T { return &value }

func TestPreserveSingleLineKeepsExtensionMode(t *testing.T) {
	old := NewUnifiedView(testPatch(), true)
	cursor := mustFirst(t, old)
	for _, extending := range []bool{false, true} {
		state := State{Selection: ptr(old.BeginSelection(cursor)), Extending: extending, Viewport: old.NewViewport(120, 10)}
		next := NewSideBySideView(testPatch(), true)
		got := Preserve(old, state, next)
		if got.Selection == nil || got.Selection.First != got.Selection.Last || got.Extending != extending {
			t.Fatalf("single-line preservation: %#v", got)
		}
		wantLine, _ := old.Line(cursor)
		gotLine, ok := next.Line(got.Cursor())
		if !ok || gotLine != wantLine {
			t.Fatalf("active line=%#v want=%#v", gotLine, wantLine)
		}
	}
}

func TestPreserveReversedRangeKeepsActiveEndpointAndScreenRow(t *testing.T) {
	p := longModelPatch()
	p.Files[0].OldPath, p.Files[0].NewPath = "long.go", "long.go"
	old := NewUnifiedView(p, true)
	first := mustFirst(t, old)
	anchor, _ := old.Search("line 20 ", first, Forward)
	active, _ := old.Search("line 10 ", first, Forward)
	selection, ok := old.ExtendSelection(old.BeginSelection(anchor), active)
	if !ok {
		t.Fatal("could not create reverse selection")
	}
	state := State{Selection: &selection, Extending: true, Viewport: old.Align(old.NewViewport(120, 10), active, Middle)}
	next := NewSideBySideView(p, true)
	got := Preserve(old, state, next)
	if got.Selection == nil || !got.Extending || got.Selection.First.Coordinate.Y <= got.Selection.Last.Coordinate.Y {
		t.Fatalf("reverse range lost direction: %#v", got)
	}
	wantLine, _ := old.Line(active)
	gotLine, _ := next.Line(got.Cursor())
	if gotLine != wantLine || got.Cursor().Coordinate.Y-got.Viewport.Top.Y != active.Coordinate.Y-state.Viewport.Top.Y {
		t.Fatalf("active endpoint or screen row changed: %#v", got)
	}
}

func TestPreserveMissingHunkCollapsesToFirstSelectableLine(t *testing.T) {
	p := coveragePatch()
	old := NewUnifiedView(p, true)
	cursor, ok := old.Search("more()", mustFirst(t, old), Forward)
	if !ok {
		t.Fatal("could not find removed hunk")
	}
	state := State{Selection: ptr(old.BeginSelection(cursor)), Extending: true, Viewport: old.NewViewport(80, 10)}
	p = coveragePatch()
	p.Files[0].Hunks = p.Files[0].Hunks[:1]
	next := NewUnifiedView(p, true)
	got := Preserve(old, state, next)
	first := mustFirst(t, next)
	if got.Selection == nil || got.Selection.First != first || got.Cursor() != first || got.Extending {
		t.Fatalf("missing-hunk fallback=%#v", got)
	}
}
