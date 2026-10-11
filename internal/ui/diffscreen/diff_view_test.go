package diffscreen

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestUnifiedNavigationSearchAndFileJumps(t *testing.T) {
	v := newUnifiedView(testPatch(), true)
	first, ok := v.first()
	testutil.Require(t, ok, "First returned no cursor")
	line, _ := v.line(first)
	testutil.Requiref(t, line.Text == "before", "first line = %q", line.Text)
	next, ok := v.move(first, forward)
	testutil.Require(t, ok, "Move returned no cursor")
	line, _ = v.line(next)
	testutil.Requiref(t, line.Kind == patch.Deletion, "next kind = %v", line.Kind)
	match, ok := v.search("added", first, forward)
	testutil.Require(t, ok, "Search returned no cursor")
	line, _ = v.line(match)
	testutil.Requiref(t, line.Text == "added one", "match = %q", line.Text)
	jumped, ok := v.jumpFile(first, forward)
	testutil.Require(t, ok, "JumpFile returned no cursor")
	file, _ := v.file(jumped)
	testutil.Requiref(t, file.DisplayPath == "second.go", "jumped file = %q", file.DisplayPath)
	{
		_, ok := v.jumpFile(jumped, forward)
		testutil.Require(t, !ok, "JumpFile wrapped unexpectedly")
	}
}

func TestSplitPairsChangeBlocksAndSupportsEmptyPanes(t *testing.T) {
	v := newSideBySideView(testPatch(), true)
	first, _ := v.first()
	removed, _ := v.move(first, forward)
	testutil.Requiref(t, removed.pane == right, "initial pane = %v", removed.pane)
	removed, ok := v.switchPane(removed, left)
	testutil.Require(t, ok, "could not switch to deletion pane")
	line, _ := v.line(removed)
	testutil.Requiref(t, line.Text == "removed one", "left line = %q", line.Text)
	added, ok := v.switchPane(removed, right)
	testutil.Require(t, ok, "paired addition missing")
	line, _ = v.line(added)
	testutil.Requiref(t, line.Text == "added one", "right line = %q", line.Text)
	secondRemoved, _ := v.move(removed, forward)
	{
		_, ok := v.switchPane(secondRemoved, right)
		testutil.Require(t, ok, "pane switch should find a nearby right line")
	}

	viewport := v.newViewport(100, 20)
	rendered := v.render(viewport, added, nil)
	testutil.Requiref(t, strings.Contains(rendered, "removed one") && strings.Contains(rendered, "added one"), "paired render missing lines: %q", rendered)
	for _, row := range strings.Split(rendered, "\n") {
		testutil.Requiref(t, !strings.Contains(row, " │ ") || lipgloss.Width(row) == 100, "rendered width = %d", lipgloss.Width(row))
	}
}

func TestSelectionLines(t *testing.T) {
	v := newUnifiedView(testPatch(), true)
	first, _ := v.first()
	last, _ := v.move(first, forward)
	last, _ = v.move(last, forward)
	selection := v.beginSelection(first)
	selection, ok := v.extendSelection(selection, last)
	testutil.Require(t, ok, "selection extension failed")
	lines := v.lines(selection)
	testutil.Requiref(t, len(lines) == 3, "selected lines = %d", len(lines))

	nextFile, _ := v.jumpFile(first, forward)
	{
		_, ok := v.extendSelection(selection, nextFile)
		testutil.Require(t, !ok, "selection crossed a hunk")
	}
}

func TestViewportAlignmentResizeAndScrolling(t *testing.T) {
	v := newUnifiedView(longPatch(), true)
	first, _ := v.first()
	cursor := first
	for range 8 {
		cursor, _ = v.move(cursor, forward)
	}
	viewport := v.newViewport(30, 5)
	viewport = v.keepVisible(viewport, cursor)
	testutil.Requiref(t, cursor.row >= viewport.top && cursor.row < viewport.top+viewport.Height, "cursor not visible: %#v %#v", cursor, viewport)
	viewport = v.align(viewport, cursor, middle)
	headerHeight := 0
	if v.hasStickyHeader(viewport.top, viewport.Height) {
		headerHeight = 1
	}
	testutil.Requiref(t, headerHeight+cursor.row-viewport.top == viewport.Height/2, "middle alignment = %#v", viewport)
	viewport = v.scrollHorizontal(viewport, 4)
	testutil.Require(t, viewport.LeftColumn != 0, "horizontal scroll did not move")
	viewport = v.resize(viewport, 20, 3)
	testutil.Requiref(t, viewport.Width == 20 && viewport.Height == 3, "resize = %#v", viewport)
	before := cursor
	viewport, cursor = v.scrollHalfPage(viewport, cursor, forward)
	testutil.Requiref(t, cursor.row >= before.row, "half page moved backward: %#v -> %#v", before, cursor)
}

func TestViewportProgressUsesVisibleBottom(t *testing.T) {
	v := newUnifiedView(longPatch(), true)
	viewport := v.newViewport(30, 5)
	{
		progress := v.viewportProgress(viewport)
		testutil.Requiref(t, progress > 0 && progress < 100, "initial progress=%d", progress)
	}
	last, _ := v.last()
	viewport = v.keepVisible(viewport, last)
	{
		progress := v.viewportProgress(viewport)
		testutil.Requiref(t, progress == 100, "final progress=%d", progress)
	}
}

func TestHalfPageScrollingMovesCursorToFileBoundaries(t *testing.T) {
	v := newUnifiedView(longPatch(), true)
	first, _ := v.first()
	last, _ := v.last()
	viewport := v.newViewport(30, 5)
	cursor := first

	for range len(longPatch().Files[0].Hunks[0].Lines) {
		viewport, cursor = v.scrollHalfPage(viewport, cursor, forward)
	}
	testutil.Requiref(t, cursor == last, "cursor after scrolling down = %#v, want %#v", cursor, last)

	for range len(longPatch().Files[0].Hunks[0].Lines) {
		viewport, cursor = v.scrollHalfPage(viewport, cursor, backward)
	}
	testutil.Requiref(t, cursor == first, "cursor after scrolling up = %#v, want %#v", cursor, first)
}

func TestFindCursorUsesSemanticIdentityAcrossChangedCoordinates(t *testing.T) {
	original := testPatch()
	oldView := newUnifiedView(original, true)
	cursor, _ := oldView.search("added one", mustFirst(t, oldView), forward)
	line, _ := oldView.line(cursor)
	changed := testPatch()
	changed.Files[0].Metadata = []string{"mode changed", "more metadata"}
	newView := newUnifiedView(changed, true)
	translated, ok := newView.findCursor(identify(oldView, cursor))
	testutil.Require(t, ok, "semantic cursor was not found")
	testutil.Require(t, translated.row != cursor.row, "cursor coordinate was reused after rows shifted")
	translatedLine, _ := newView.line(translated)
	testutil.Requiref(t, translatedLine == line, "translated line = %#v, want %#v", translatedLine, line)
}

func TestSplitViewWithOnlyDeletionsStartsInLeftPane(t *testing.T) {
	p := patch.Patch{Files: []patch.File{{DisplayPath: "deleted.go", Hunks: []patch.Hunk{{Header: "@@", Lines: []patch.Line{{Kind: patch.Deletion, Text: "gone", OldNumber: 1}}}}}}}
	v := newSideBySideView(p, true)
	cursor, ok := v.first()
	testutil.Requiref(t, ok && cursor.pane == left, "first cursor = %#v, %v", cursor, ok)
	line, _ := v.line(cursor)
	testutil.Requiref(t, line.Text == "gone", "first line = %q", line.Text)
}

func TestFindCursorFallsBackNearRemovedLine(t *testing.T) {
	original := testPatch()
	oldView := newUnifiedView(original, true)
	cursor, _ := oldView.search("removed two", mustFirst(t, oldView), forward)
	changed := testPatch()
	changed.Files[0].Hunks[0].Lines = changed.Files[0].Hunks[0].Lines[:2]
	newView := newUnifiedView(changed, true)
	fallback, ok := newView.findCursor(identify(oldView, cursor))
	testutil.Require(t, ok, "nearby cursor was not found")
	fallbackLine, _ := newView.line(fallback)
	testutil.Requiref(t, fallbackLine.Kind == patch.Deletion, "fallback kind = %v", fallbackLine.Kind)
}

func mustFirst(t *testing.T, v *diffView) diffCursor {
	t.Helper()
	cursor, ok := v.first()
	testutil.Require(t, ok, "no first cursor")
	return cursor
}

func testPatch() patch.Patch {
	return patch.Patch{Root: "/repo", Files: []patch.File{
		{DisplayPath: "first.go", OldPath: "first.go", NewPath: "first.go", Hunks: []patch.Hunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patch.Line{
			{Kind: patch.Context, Text: "before", OldNumber: 1, NewNumber: 1},
			{Kind: patch.Deletion, Text: "removed one", OldNumber: 2},
			{Kind: patch.Deletion, Text: "removed two", OldNumber: 3},
			{Kind: patch.Addition, Text: "added one", NewNumber: 2},
			{Kind: patch.Context, Text: "after", OldNumber: 4, NewNumber: 3},
		}}}},
		{DisplayPath: "second.go", OldPath: "second.go", NewPath: "second.go", Hunks: []patch.Hunk{{Header: "@@ -1 +1 @@", Lines: []patch.Line{{Kind: patch.Addition, Text: "other", NewNumber: 1}}}}},
	}}
}

func longPatch() patch.Patch {
	lines := make([]patch.Line, 20)
	for index := range lines {
		lines[index] = patch.Line{Kind: patch.Context, Text: strings.Repeat("long", 20), OldNumber: patch.LineNumber(index + 1), NewNumber: patch.LineNumber(index + 1)}
	}
	return patch.Patch{Files: []patch.File{{DisplayPath: "long.go", Hunks: []patch.Hunk{{Header: "@@", Lines: lines}}}}}
}

func newUnifiedView(p patch.Patch, dark bool) *diffView {
	return newDiffView(p, false, dark)
}

func newSideBySideView(p patch.Patch, dark bool) *diffView {
	return newDiffView(p, true, dark)
}
