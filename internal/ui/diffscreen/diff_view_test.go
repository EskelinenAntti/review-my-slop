package diffscreen

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

func TestUnifiedNavigationSearchAndFileJumps(t *testing.T) {
	v := newUnifiedView(testPatch(), true)
	first, ok := v.first()
	if !ok {
		t.Fatal("First returned no cursor")
	}
	line, _ := v.line(first)
	if line.Text != "before" {
		t.Fatalf("first line = %q", line.Text)
	}
	next, ok := v.move(first, forward)
	if !ok {
		t.Fatal("Move returned no cursor")
	}
	line, _ = v.line(next)
	if line.Kind != patch.Deletion {
		t.Fatalf("next kind = %v", line.Kind)
	}
	match, ok := v.search("added", first, forward)
	if !ok {
		t.Fatal("Search returned no cursor")
	}
	line, _ = v.line(match)
	if line.Text != "added one" {
		t.Fatalf("match = %q", line.Text)
	}
	jumped, ok := v.jumpFile(first, forward)
	if !ok {
		t.Fatal("JumpFile returned no cursor")
	}
	file, _ := v.file(jumped)
	if file.DisplayPath != "second.go" {
		t.Fatalf("jumped file = %q", file.DisplayPath)
	}
	if _, ok := v.jumpFile(jumped, forward); ok {
		t.Fatal("JumpFile wrapped unexpectedly")
	}
}

func TestSplitPairsChangeBlocksAndSupportsEmptyPanes(t *testing.T) {
	v := newSideBySideView(testPatch(), true)
	first, _ := v.first()
	removed, _ := v.move(first, forward)
	if removed.pane != right {
		t.Fatalf("initial pane = %v", removed.pane)
	}
	removed, ok := v.switchPane(removed, left)
	if !ok {
		t.Fatal("could not switch to deletion pane")
	}
	line, _ := v.line(removed)
	if line.Text != "removed one" {
		t.Fatalf("left line = %q", line.Text)
	}
	added, ok := v.switchPane(removed, right)
	if !ok {
		t.Fatal("paired addition missing")
	}
	line, _ = v.line(added)
	if line.Text != "added one" {
		t.Fatalf("right line = %q", line.Text)
	}
	secondRemoved, _ := v.move(removed, forward)
	if _, ok := v.switchPane(secondRemoved, right); !ok {
		t.Fatal("pane switch should find a nearby right line")
	}

	viewport := v.newViewport(100, 20)
	rendered := v.render(viewport, added, nil)
	if !strings.Contains(rendered, "removed one") || !strings.Contains(rendered, "added one") {
		t.Fatalf("paired render missing lines: %q", rendered)
	}
	for _, row := range strings.Split(rendered, "\n") {
		if strings.Contains(row, " │ ") && lipgloss.Width(row) != 100 {
			t.Fatalf("rendered width = %d", lipgloss.Width(row))
		}
	}
}

func TestSelectionLines(t *testing.T) {
	v := newUnifiedView(testPatch(), true)
	first, _ := v.first()
	last, _ := v.move(first, forward)
	last, _ = v.move(last, forward)
	selection := v.beginSelection(first)
	selection, ok := v.extendSelection(selection, last)
	if !ok {
		t.Fatal("selection extension failed")
	}
	lines := v.lines(selection)
	if len(lines) != 3 {
		t.Fatalf("selected lines = %d", len(lines))
	}

	nextFile, _ := v.jumpFile(first, forward)
	if _, ok := v.extendSelection(selection, nextFile); ok {
		t.Fatal("selection crossed a hunk")
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
	if cursor.row < viewport.top || cursor.row >= viewport.top+viewport.Height {
		t.Fatalf("cursor not visible: %#v %#v", cursor, viewport)
	}
	viewport = v.align(viewport, cursor, middle)
	headerHeight := 0
	if v.hasStickyHeader(viewport.top, viewport.Height) {
		headerHeight = 1
	}
	if headerHeight+cursor.row-viewport.top != viewport.Height/2 {
		t.Fatalf("middle alignment = %#v", viewport)
	}
	viewport = v.scrollHorizontal(viewport, 4)
	if viewport.LeftColumn == 0 {
		t.Fatal("horizontal scroll did not move")
	}
	viewport = v.resize(viewport, 20, 3)
	if viewport.Width != 20 || viewport.Height != 3 {
		t.Fatalf("resize = %#v", viewport)
	}
	before := cursor
	viewport, cursor = v.scrollHalfPage(viewport, cursor, forward)
	if cursor.row < before.row {
		t.Fatalf("half page moved backward: %#v -> %#v", before, cursor)
	}
}

func TestViewportProgressUsesVisibleBottom(t *testing.T) {
	v := newUnifiedView(longPatch(), true)
	viewport := v.newViewport(30, 5)
	if progress := v.viewportProgress(viewport); progress <= 0 || progress >= 100 {
		t.Fatalf("initial progress=%d", progress)
	}
	last, _ := v.last()
	viewport = v.keepVisible(viewport, last)
	if progress := v.viewportProgress(viewport); progress != 100 {
		t.Fatalf("final progress=%d", progress)
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
	if cursor != last {
		t.Fatalf("cursor after scrolling down = %#v, want %#v", cursor, last)
	}

	for range len(longPatch().Files[0].Hunks[0].Lines) {
		viewport, cursor = v.scrollHalfPage(viewport, cursor, backward)
	}
	if cursor != first {
		t.Fatalf("cursor after scrolling up = %#v, want %#v", cursor, first)
	}
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
	if !ok {
		t.Fatal("semantic cursor was not found")
	}
	if translated.row == cursor.row {
		t.Fatal("cursor coordinate was reused after rows shifted")
	}
	translatedLine, _ := newView.line(translated)
	if translatedLine != line {
		t.Fatalf("translated line = %#v, want %#v", translatedLine, line)
	}
}

func TestSplitViewWithOnlyDeletionsStartsInLeftPane(t *testing.T) {
	p := patch.Patch{Files: []patch.File{{DisplayPath: "deleted.go", Hunks: []patch.Hunk{{Header: "@@", Lines: []patch.Line{{Kind: patch.Deletion, Text: "gone", OldNumber: 1}}}}}}}
	v := newSideBySideView(p, true)
	cursor, ok := v.first()
	if !ok || cursor.pane != left {
		t.Fatalf("first cursor = %#v, %v", cursor, ok)
	}
	line, _ := v.line(cursor)
	if line.Text != "gone" {
		t.Fatalf("first line = %q", line.Text)
	}
}

func TestFindCursorFallsBackNearRemovedLine(t *testing.T) {
	original := testPatch()
	oldView := newUnifiedView(original, true)
	cursor, _ := oldView.search("removed two", mustFirst(t, oldView), forward)
	changed := testPatch()
	changed.Files[0].Hunks[0].Lines = changed.Files[0].Hunks[0].Lines[:2]
	newView := newUnifiedView(changed, true)
	fallback, ok := newView.findCursor(identify(oldView, cursor))
	if !ok {
		t.Fatal("nearby cursor was not found")
	}
	fallbackLine, _ := newView.line(fallback)
	if fallbackLine.Kind != patch.Deletion {
		t.Fatalf("fallback kind = %v", fallbackLine.Kind)
	}
}

func mustFirst(t *testing.T, v *diffView) diffCursor {
	t.Helper()
	cursor, ok := v.first()
	if !ok {
		t.Fatal("no first cursor")
	}
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
