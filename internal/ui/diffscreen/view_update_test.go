package diffscreen

import (
	"fmt"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestPreserveTranslatesCursorSelectionAndViewport(t *testing.T) {
	old := newUnifiedPatchView(twoFileReviewPatch(), true)
	firstCursor := mustFirstCursor(t, old)
	cursor, ok := old.searchFromCursor("added one", firstCursor, forward)
	testutil.RequireEqual(t, ok, true, "cursor not found")

	selection := beginRange(firstCursor)
	selection, ok = old.extendRange(selection, cursor)
	testutil.RequireEqual(t, ok, true, "selection not created")

	viewport := old.newViewport(100, 4)
	viewport = old.alignCursor(viewport, cursor, middle)
	screen := &View{
		projection: old,
		cursor:     cursor,
		selection:  &selection,
		viewport:   viewport,
	}

	next := newSplitPatchView(twoFileReviewPatch(), true)
	screen.replaceProjection(next)
	preserved := screen
	testutil.RequireEqual(t, next.validCursor(preserved.cursor), true,
		"cursor was not preserved",
	)

	line, ok := next.lineAtCursor(preserved.cursor)
	testutil.RequireEqual(t, ok, true,
		fmt.Sprintf("cursor line = %#v, ok=%v", line, ok),
	)
	testutil.RequireEqual(t, line.Text, "added one",
		fmt.Sprintf("cursor line = %#v, ok=%v", line, ok),
	)
	testutil.RequireEqual(t, preserved.selection != nil, true,
		fmt.Sprintf("selection = %#v", preserved.selection),
	)

	firstLine, firstOK := next.lineAtCursor(preserved.selection.First)
	lastLine, lastOK := next.lineAtCursor(preserved.selection.Last)
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
	old := newUnifiedPatchView(twoFileReviewPatch(), true)
	cursor := mustFirstCursor(t, old)
	screen := &View{
		projection: old,
		cursor:     cursor,
		selection:  pointerTo(beginRange(cursor)),
		viewport:   old.newViewport(80, 10),
	}

	screen.replaceProjection(newUnifiedPatchView(patch.Patch{}, true))
	preserved := screen
	testutil.RequireEqual(t,
		!preserved.projection.validCursor(preserved.cursor), true,
		fmt.Sprintf("state = %#v", preserved),
	)
	testutil.RequireEqual(t, preserved.selection, nil,
		fmt.Sprintf("state = %#v", preserved),
	)
}

func TestPreserveClampsHorizontalOffsetForShorterLines(t *testing.T) {
	p := manyContextLinesPatch()
	p.Files[0].OldPath, p.Files[0].NewPath = "long.go", "long.go"
	old := newUnifiedPatchView(p, true)
	cursor := mustFirstCursor(t, old)
	viewport := old.scrollHorizontal(old.newViewport(40, 10), 30)
	testutil.RequireEqual(t, viewport.LeftColumn != 0, true,
		"fixture does not scroll horizontally",
	)

	short := twoFileReviewPatch()
	preserved := &View{projection: old, cursor: cursor, viewport: viewport}
	preserved.replaceProjection(newUnifiedPatchView(short, true))
	testutil.RequireEqual(t, preserved.viewport.LeftColumn, 0,
		fmt.Sprintf("horizontal offset = %d", preserved.viewport.LeftColumn),
	)
}
