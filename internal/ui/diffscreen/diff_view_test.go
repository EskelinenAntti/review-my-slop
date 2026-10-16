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
	first, ok := v.firstSelectableLine()
	testutil.RequireValuesEqual(t, ok, true, "%s", "First returned no cursor")

	line, _ := v.patchLineForPosition(first)
	testutil.RequireValuesEqual(
		t,
		line.Text,
		"before",
		"first line = %q",
		line.Text,
	)

	next, ok := v.moveInDirection(first, forward)
	testutil.RequireValuesEqual(t, ok, true, "%s", "Move returned no cursor")

	line, _ = v.patchLineForPosition(next)
	testutil.RequireValuesEqual(t, line.Kind, patch.Deletion, "next kind = %v",
		line.Kind,
	)

	match, ok := v.findTextInChanges("added", first, forward)
	testutil.RequireValuesEqual(t, ok, true, "%s", "Search returned no cursor")

	line, _ = v.patchLineForPosition(match)
	testutil.RequireValuesEqual(
		t,
		line.Text,
		"added one",
		"match = %q",
		line.Text,
	)

	jumped, ok := v.moveFocusToAdjacentFile(first, forward)
	testutil.RequireValuesEqual(
		t,
		ok,
		true,
		"%s",
		"JumpFile returned no cursor",
	)

	file, _ := v.fileForLine(jumped)
	testutil.RequireValuesEqual(
		t,
		file.DisplayPath,
		"second.go",
		"jumped file = %q",
		file.DisplayPath,
	)

	if _, ok := v.moveFocusToAdjacentFile(jumped, forward); ok {
		t.Fatal("JumpFile wrapped unexpectedly")
	}
}

func TestSplitPairsChangeBlocksAndSupportsEmptyPanes(t *testing.T) {
	v := newSideBySideView(testPatch(), true)
	first, _ := v.firstSelectableLine()
	removed, _ := v.moveInDirection(first, forward)
	testutil.RequireValuesEqual(t, removed.pane, right, "initial pane = %v",
		removed.pane,
	)

	removed, ok := v.switchToPane(removed, left)
	testutil.RequireValuesEqual(t, ok, true, "%s",
		"could not switch to deletion pane",
	)

	line, _ := v.patchLineForPosition(removed)
	testutil.RequireValuesEqual(t, line.Text, "removed one", "left line = %q",
		line.Text,
	)

	added, ok := v.switchToPane(removed, right)
	testutil.RequireValuesEqual(t, ok, true, "%s", "paired addition missing")

	line, _ = v.patchLineForPosition(added)
	testutil.RequireValuesEqual(t, line.Text, "added one", "right line = %q",
		line.Text,
	)

	secondRemoved, _ := v.moveInDirection(removed, forward)
	if _, ok := v.switchToPane(secondRemoved, right); !ok {
		t.Fatal("pane switch should find a nearby right line")
	}

	viewport := v.newVisibleArea(100, 20)
	rendered := v.formatVisibleChanges(viewport, added, nil)
	testutil.RequireValuesEqual(
		t,
		strings.Contains(rendered, "removed one"),
		true,
		"paired render missing lines: %q",
		rendered,
	)
	testutil.RequireValuesEqual(
		t,
		strings.Contains(rendered, "added one"),
		true,
		"paired render missing lines: %q",
		rendered,
	)

	for _, row := range strings.Split(rendered, "\n") {
		testutil.RequireValuesEqual(t,
			!strings.Contains(row, " │ ") || lipgloss.Width(row) == 100, true,
			"rendered width = %d", lipgloss.Width(row),
		)
	}
}

func TestSelectionLines(t *testing.T) {
	v := newUnifiedView(testPatch(), true)
	first, _ := v.firstSelectableLine()
	last, _ := v.moveInDirection(first, forward)
	last, _ = v.moveInDirection(last, forward)
	selection := beginSelection(first)
	selection, ok := v.extendSelection(selection, last)
	testutil.RequireValuesEqual(t, ok, true, "%s", "selection extension failed")

	lines := v.selectedLines(selection)
	testutil.RequireValuesEqual(
		t,
		len(lines),
		3,
		"selected lines = %d",
		len(lines),
	)

	nextFile, _ := v.moveFocusToAdjacentFile(first, forward)
	if _, ok := v.extendSelection(selection, nextFile); ok {
		t.Fatal("selection crossed a hunk")
	}
}

func TestViewportAlignmentResizeAndScrolling(t *testing.T) {
	v := newUnifiedView(longPatch(), true)
	first, _ := v.firstSelectableLine()
	cursor := first
	for range 8 {
		cursor, _ = v.moveInDirection(cursor, forward)
	}
	viewport := v.newVisibleArea(30, 5)
	viewport = v.keepFocusedLineVisible(viewport, cursor)
	testutil.RequireValuesEqual(t, cursor.row >= viewport.top, true,
		"cursor not visible: %#v %#v", cursor, viewport,
	)
	testutil.RequireValuesEqual(
		t,
		cursor.row < viewport.top+viewport.Height,
		true,
		"cursor not visible: %#v %#v",
		cursor,
		viewport,
	)

	viewport = v.positionFocusedLine(viewport, cursor, middle)
	headerHeight := 0
	if v.fileHeaderStaysAtTop(viewport.top, viewport.Height) {
		headerHeight = 1
	}
	testutil.RequireValuesEqual(t, headerHeight+cursor.row-viewport.top,
		viewport.Height/2, "middle alignment = %#v", viewport,
	)

	viewport = v.scrollVisibleAreaSideways(viewport, 4)
	testutil.RequireValuesEqual(t, viewport.LeftColumn != 0, true, "%s",
		"horizontal scroll did not move",
	)

	viewport = v.resize(viewport, 20, 3)
	testutil.RequireValuesEqual(t, viewport.Width, 20, "resize = %#v", viewport)
	testutil.RequireValuesEqual(t, viewport.Height, 3, "resize = %#v", viewport)

	before := cursor
	viewport, cursor = v.moveFocusedLineHalfPage(viewport, cursor, forward)
	testutil.RequireValuesEqual(t, cursor.row >= before.row, true,
		"half page moved backward: %#v -> %#v", before, cursor,
	)
}

func TestViewportProgressUsesVisibleBottom(t *testing.T) {
	v := newUnifiedView(longPatch(), true)
	viewport := v.newVisibleArea(30, 5)
	progress := v.visibleContentProgressPercent(viewport)
	if progress <= 0 || progress >= 100 {
		t.Fatalf("initial progress=%d", progress)
	}
	last, _ := v.lastSelectableLine()
	viewport = v.keepFocusedLineVisible(viewport, last)
	if progress := v.visibleContentProgressPercent(viewport); progress != 100 {
		t.Fatalf("final progress=%d", progress)
	}
}

func TestHalfPageScrollingMovesCursorToFileBoundaries(t *testing.T) {
	v := newUnifiedView(longPatch(), true)
	first, _ := v.firstSelectableLine()
	last, _ := v.lastSelectableLine()
	viewport := v.newVisibleArea(30, 5)
	cursor := first

	for range len(longPatch().Files[0].Hunks[0].Lines) {
		viewport, cursor = v.moveFocusedLineHalfPage(viewport, cursor, forward)
	}
	testutil.RequireValuesEqual(t, cursor, last,
		"cursor after scrolling down = %#v, want %#v", cursor, last,
	)

	for range len(longPatch().Files[0].Hunks[0].Lines) {
		viewport, cursor = v.moveFocusedLineHalfPage(viewport, cursor, backward)
	}
	testutil.RequireValuesEqual(t, cursor, first,
		"cursor after scrolling up = %#v, want %#v", cursor, first,
	)
}

func TestFindCursorUsesSemanticIdentityAcrossChangedCoordinates(t *testing.T) {
	original := testPatch()
	oldView := newUnifiedView(original, true)
	cursor, _ := oldView.findTextInChanges(
		"added one",
		mustFirst(t, oldView),
		forward,
	)
	line, _ := oldView.patchLineForPosition(cursor)
	changed := testPatch()
	changed.Files[0].Metadata = []string{"mode changed", "more metadata"}
	newView := newUnifiedView(changed, true)
	translated, ok := newView.findSameLineAfterChanges(
		describeFocusedLine(oldView, cursor),
	)
	testutil.RequireValuesEqual(
		t,
		ok,
		true,
		"%s",
		"semantic cursor was not found",
	)
	testutil.RequireValuesEqual(t, translated.row != cursor.row, true, "%s",
		"cursor coordinate was reused after rows shifted",
	)

	translatedLine, _ := newView.patchLineForPosition(translated)
	testutil.RequireValuesEqual(t, translatedLine, line,
		"translated line = %#v, want %#v", translatedLine, line,
	)
}

func TestSplitViewWithOnlyDeletionsStartsInLeftPane(t *testing.T) {
	p := patch.Patch{
		Files: []patch.File{
			{
				DisplayPath: "deleted.go",
				Hunks: []patch.Hunk{
					{
						Header: "@@",
						Lines: []patch.Line{
							{Kind: patch.Deletion, Text: "gone", OldNumber: 1},
						},
					},
				},
			},
		},
	}
	v := newSideBySideView(p, true)
	cursor, ok := v.firstSelectableLine()
	testutil.RequireValuesEqual(
		t,
		ok,
		true,
		"first cursor = %#v, %v",
		cursor,
		ok,
	)
	testutil.RequireValuesEqual(t, cursor.pane, left, "first cursor = %#v, %v",
		cursor, ok,
	)

	line, _ := v.patchLineForPosition(cursor)
	testutil.RequireValuesEqual(
		t,
		line.Text,
		"gone",
		"first line = %q",
		line.Text,
	)
}

func TestFindCursorFallsBackNearRemovedLine(t *testing.T) {
	original := testPatch()
	oldView := newUnifiedView(original, true)
	cursor, _ := oldView.findTextInChanges(
		"removed two",
		mustFirst(t, oldView),
		forward,
	)
	changed := testPatch()
	changed.Files[0].Hunks[0].Lines = changed.Files[0].Hunks[0].Lines[:2]
	newView := newUnifiedView(changed, true)
	fallback, ok := newView.findSameLineAfterChanges(
		describeFocusedLine(oldView, cursor),
	)
	testutil.RequireValuesEqual(
		t,
		ok,
		true,
		"%s",
		"nearby cursor was not found",
	)

	fallbackLine, _ := newView.patchLineForPosition(fallback)
	testutil.RequireValuesEqual(t, fallbackLine.Kind, patch.Deletion,
		"fallback kind = %v", fallbackLine.Kind,
	)
}

func mustFirst(t *testing.T, v *diffView) diffCursor {
	t.Helper()
	cursor, ok := v.firstSelectableLine()
	testutil.RequireValuesEqual(t, ok, true, "%s", "no first cursor")

	return cursor
}

func testPatch() patch.Patch {
	return patch.Patch{Root: "/repo", Files: []patch.File{
		{
			DisplayPath: "first.go",
			OldPath:     "first.go",
			NewPath:     "first.go",
			Hunks: []patch.Hunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patch.Line{
				{
					Kind:      patch.Context,
					Text:      "before",
					OldNumber: 1,
					NewNumber: 1,
				},
				{Kind: patch.Deletion, Text: "removed one", OldNumber: 2},
				{Kind: patch.Deletion, Text: "removed two", OldNumber: 3},
				{Kind: patch.Addition, Text: "added one", NewNumber: 2},
				{
					Kind:      patch.Context,
					Text:      "after",
					OldNumber: 4,
					NewNumber: 3,
				},
			}}},
		},
		{
			DisplayPath: "second.go",
			OldPath:     "second.go",
			NewPath:     "second.go",
			Hunks: []patch.Hunk{
				{
					Header: "@@ -1 +1 @@",
					Lines: []patch.Line{
						{Kind: patch.Addition, Text: "other", NewNumber: 1},
					},
				},
			},
		},
	}}
}

func longPatch() patch.Patch {
	lines := make([]patch.Line, 20)
	for index := range lines {
		lines[index] = patch.Line{
			Kind:      patch.Context,
			Text:      strings.Repeat("long", 20),
			OldNumber: patch.LineNumber(index + 1),
			NewNumber: patch.LineNumber(index + 1),
		}
	}
	return patch.Patch{
		Files: []patch.File{
			{
				DisplayPath: "long.go",
				Hunks:       []patch.Hunk{{Header: "@@", Lines: lines}},
			},
		},
	}
}

func newUnifiedView(p patch.Patch, dark bool) *diffView {
	return createChangesView(p, Options{Dark: dark})
}

func newSideBySideView(p patch.Patch, dark bool) *diffView {
	return createChangesView(p, Options{SideBySide: true, Dark: dark})
}
