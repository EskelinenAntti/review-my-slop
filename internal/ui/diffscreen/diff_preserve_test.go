package diffscreen

import (
	"fmt"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

func TestPreserveTranslatesCursorSelectionAndViewport(t *testing.T) {
	old := newUnifiedView(testPatch(), true)
	first := mustFirst(t, old)
	cursor, ok := old.search("added one", first, forward)
	if !ok {
		t.Fatal("cursor not found")
	}
	selection := old.beginSelection(first)
	selection, ok = old.extendSelection(selection, cursor)
	if !ok {
		t.Fatal("selection not created")
	}
	viewport := old.newViewport(100, 4)
	viewport = old.align(viewport, cursor, middle)
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

func TestUpdatePreservesCursorAfterLineDeletion(t *testing.T) {
	early := patch.Hunk{Header: "@@ -1,0 +1 @@", Lines: []patch.Line{
		{Kind: patch.Addition, Text: "early change", NewNumber: 1},
	}}
	before := patch.Line{Kind: patch.Context, Text: "before", OldNumber: 50, NewNumber: 50}
	added := patch.Line{Kind: patch.Addition, Text: "removed addition", NewNumber: 51}
	after := patch.Line{Kind: patch.Context, Text: "after", OldNumber: 51, NewNumber: 52}
	target := patch.Line{Kind: patch.Addition, Text: "target", NewNumber: 53}
	late := patch.Hunk{Header: "@@ -100,0 +100 @@", Lines: []patch.Line{
		{Kind: patch.Addition, Text: "late change", NewNumber: 100},
	}}
	makePatch := func(hunks []patch.Hunk) patch.Patch {
		return patch.Patch{Files: []patch.File{{OldPath: "file.go", NewPath: "file.go", Hunks: hunks}}}
	}
	original := makePatch([]patch.Hunk{early, {
		Header: "@@ -50,2 +50,4 @@", Lines: []patch.Line{before, added, after, target},
	}, late})
	shiftedAfter, shiftedTarget := after, target
	shiftedAfter.NewNumber--
	shiftedTarget.NewNumber--
	for _, split := range []bool{false, true} {
		for _, tc := range []struct {
			name  string
			hunks []patch.Hunk
			want  patch.Line
		}{
			{"line before cursor deleted", []patch.Hunk{early, {
				Header: "@@ -50,2 +50,3 @@", Lines: []patch.Line{before, shiftedAfter, shiftedTarget},
			}, late}, shiftedTarget},
			{"cursor line deleted", []patch.Hunk{early, {
				Header: "@@ -50,2 +50,3 @@", Lines: []patch.Line{before, added, after},
			}, late}, after},
			{"cursor hunk removed", []patch.Hunk{early, late}, late.Lines[0]},
		} {
			t.Run(fmt.Sprintf("%s/split=%v", tc.name, split), func(t *testing.T) {
				v := New(original, Options{SideBySide: split})
				v.Resize(120, 4)
				v.BeginSearch()
				v.InsertSearch("target")
				v.AcceptSearch()
				_, line, ok := v.Current()
				if !ok || line != target {
					t.Fatalf("initial cursor = %#v, ok=%v", line, ok)
				}
				v.Update(makePatch(tc.hunks))
				_, line, ok = v.Current()
				if !ok || line != tc.want {
					t.Fatalf("restored cursor = %#v, ok=%v, want %#v", line, ok, tc.want)
				}
				if v.cursor.row < v.viewport.top || v.cursor.row >= v.viewport.top+v.view.contentHeight(v.viewport) {
					t.Fatal("restored cursor is outside the viewport")
				}
			})
		}
	}
}

func ptr[T any](value T) *T { return &value }
