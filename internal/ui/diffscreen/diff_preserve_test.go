package diffscreen

import (
	"fmt"
	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
	"testing"
)

func TestPreserveTranslatesCursorSelectionAndViewport(t *testing.T) {
	old := newUnifiedView(testPatch(), true)
	first := mustFirst(t, old)
	cursor, ok := old.search("added one", first, forward)
	testutil.RequireEqual(t, ok, true, "cursor not found")

	selection := beginSelection(first)
	selection, ok = old.extendSelection(selection, cursor)
	testutil.RequireEqual(t, ok, true, "selection not created")

	viewport := old.newViewport(100, 4)
	viewport = old.align(viewport, cursor, middle)
	screen := &View{
		view:      old,
		cursor:    cursor,
		selection: &selection,
		viewport:  viewport,
	}

	next := newSideBySideView(testPatch(), true)
	screen.replaceView(next)
	preserved := screen
	testutil.RequireEqual(t, next.valid(preserved.cursor), true,
		"cursor was not preserved",
	)

	line, ok := next.line(preserved.cursor)
	testutil.RequireEqual(t, ok, true,
		fmt.Sprintf("cursor line = %#v, ok=%v", line, ok),
	)
	testutil.RequireEqual(t, line.Text, "added one",
		fmt.Sprintf("cursor line = %#v, ok=%v", line, ok),
	)
	testutil.RequireEqual(t, preserved.selection != nil, true,
		fmt.Sprintf("selection = %#v", preserved.selection),
	)

	firstLine, firstOK := next.line(preserved.selection.First)
	lastLine, lastOK := next.line(preserved.selection.Last)
	testutil.RequireEqual(t, firstOK, true,
		fmt.Sprintf("selection endpoints = %#v, %#v", firstLine, lastLine),
	)
	testutil.RequireEqual(t, lastOK, true,
		fmt.Sprintf("selection endpoints = %#v, %#v", firstLine, lastLine),
	)
	testutil.RequireEqual(t, firstLine.Text, "before",
		fmt.Sprintf("selection endpoints = %#v, %#v", firstLine, lastLine),
	)
	testutil.RequireEqual(t, lastLine.Text, "added one",
		fmt.Sprintf("selection endpoints = %#v, %#v", firstLine, lastLine),
	)

	got := preserved.cursor.row - preserved.viewport.top
	want := cursor.row - viewport.top
	if got != want {
		t.Fatalf("screen row = %d, want %d", got, want)
	}
}

func TestPreserveReturnsEmptyStateForEmptyView(t *testing.T) {
	old := newUnifiedView(testPatch(), true)
	cursor := mustFirst(t, old)
	screen := &View{
		view:      old,
		cursor:    cursor,
		selection: ptr(beginSelection(cursor)),
		viewport:  old.newViewport(80, 10),
	}

	screen.replaceView(newUnifiedView(patch.Patch{}, true))
	preserved := screen
	testutil.RequireEqual(t, !preserved.view.valid(preserved.cursor), true,
		fmt.Sprintf("state = %#v", preserved),
	)
	testutil.RequireEqual(t, preserved.selection, nil,
		fmt.Sprintf("state = %#v", preserved),
	)
}

func TestPreserveClampsHorizontalOffsetForShorterLines(t *testing.T) {
	p := longPatch()
	p.Files[0].OldPath, p.Files[0].NewPath = "long.go", "long.go"
	old := newUnifiedView(p, true)
	cursor := mustFirst(t, old)
	viewport := old.scrollHorizontal(old.newViewport(40, 10), 30)
	testutil.RequireEqual(t, viewport.LeftColumn != 0, true,
		"fixture does not scroll horizontally",
	)

	short := testPatch()
	preserved := &View{view: old, cursor: cursor, viewport: viewport}
	preserved.replaceView(newUnifiedView(short, true))
	testutil.RequireEqual(t, preserved.viewport.LeftColumn, 0,
		fmt.Sprintf("horizontal offset = %d", preserved.viewport.LeftColumn),
	)
}
