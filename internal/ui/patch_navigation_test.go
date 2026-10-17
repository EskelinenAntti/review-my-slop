package ui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	patches "github.com/eskelinenantti/review-my-slop/internal/patch"
)

func TestDiffscreenUnifiedNavigationSearchAndFileJumps(t *testing.T) {
	v := diffscreenTestViewWithTheme(diffscreenTwoFileSelectionPatch(), true)
	first, ok := diffscreenSearchCandidateCursor(v)
	requireEqual(t, ok, true, "%s", "First returned no cursor")

	_, line, _ := patchLineAtCursor(v, first)
	requireEqual(t, line.Text, "before", "first line = %q", line.Text)

	next, ok := scanCursor(v, first, scanForward)
	requireEqual(t, ok, true, "%s", "Move returned no cursor")

	_, line, _ = patchLineAtCursor(v, next)
	requireEqual(t, line.Kind, patches.Deletion, "next kind = %v", line.Kind)

	match, ok := findSearchCursor(v, "added", first)
	requireEqual(t, ok, true, "%s", "Search returned no cursor")

	_, line, _ = patchLineAtCursor(v, match)
	requireEqual(t, line.Text, "added one", "match = %q", line.Text)

	jumped, ok := jumpFile(v, first, scanForward)
	requireEqual(t, ok, true, "%s", "JumpFile returned no cursor")

	file, _, _ := patchLineAtCursor(v, jumped)
	requireEqual(t, file.DisplayPath, "second.go", "jumped file = %q",
		file.DisplayPath,
	)

	_, wrapped := nextCursor(v, jumped.pane, jumped.row)
	requireEqual(t, wrapped, false, "%s", "JumpFile wrapped unexpectedly")
}

func TestDiffscreenSplitPairsChangeBlocksAndSupportsEmptyPanes(t *testing.T) {
	v := diffscreenTestSplitViewWithTheme(
		diffscreenTwoFileSelectionPatch(), true,
	)
	first, ok := diffscreenSearchCandidateCursor(v)
	requireEqual(t, ok, true, "%s", "no first cursor")
	removed, ok := nextCursor(v, first.pane, first.row)
	requireEqual(t, ok, true, "%s", "no next cursor")
	requireEqual(t, removed.pane, right,
		"initial pane = %v", removed.pane,
	)

	removed, ok = closestCursorInPane(left, v, removed)
	requireEqual(t, ok, true, "%s", "could not switch to deletion pane")
	_, line, _ := patchLineAtCursor(v, removed)
	requireEqual(t, line.Text, "removed one", "left line = %q", line.Text)

	added, ok := closestCursorInPane(right, v, removed)
	requireEqual(t, ok, true, "%s", "paired addition missing")
	_, line, _ = patchLineAtCursor(v, added)
	requireEqual(t, line.Text, "added one", "right line = %q", line.Text)

	secondRemoved, ok := nextCursor(v, removed.pane, removed.row)
	requireEqual(t, ok, true, "%s", "second deletion missing")
	_, ok = closestCursorInPane(right, v, secondRemoved)
	requireEqual(t, ok, true, "%s",
		"pane switch should find a nearby right line",
	)

	viewport := initialViewport(v, 100, 20)
	rendered := renderPatch(v, viewport, added, nil)
	requireEqual(t, strings.Contains(rendered, "removed one"), true,
		"paired render missing lines: %q", rendered,
	)
	requireEqual(t, strings.Contains(rendered, "added one"), true,
		"paired render missing lines: %q", rendered,
	)
	for _, row := range strings.Split(rendered, "\n") {
		requireEqual(
			t, !strings.Contains(row, " │ ") || lipgloss.Width(row) == 100,
			true, "rendered width = %d", lipgloss.Width(row),
		)
	}
}

func TestDiffscreenSelectionLines(t *testing.T) {
	v := diffscreenTestViewWithTheme(diffscreenTwoFileSelectionPatch(), true)
	first, ok := diffscreenSearchCandidateCursor(v)
	requireEqual(t, ok, true, "%s", "no first cursor")
	last, ok := nextCursor(v, first.pane, first.row)
	requireEqual(t, ok, true, "%s", "no second cursor")
	last, ok = nextCursor(v, last.pane, last.row)
	requireEqual(t, ok, true, "%s", "no third cursor")

	selection, ok := selectionForDrag(v, first, last)
	requireEqual(t, ok, true, "%s", "selection extension failed")
	lines := linesInSelection(v, selection)
	requireEqual(t, len(lines), 3, "selected lines = %d", len(lines))

	nextFile, ok := jumpFile(v, first, scanForward)
	requireEqual(t, ok, true, "%s", "next file cursor missing")
	_, ok = selectionForDrag(v, first, nextFile)
	requireEqual(t, ok, false, "%s", "selection crossed a hunk")
}

func TestDiffscreenViewportAlignmentResizeAndScrolling(t *testing.T) {
	v := newDiffscreenTestView(
		diffscreenLongContextPatch(), Options{Dark: true},
	)
	cursor, ok := diffscreenSearchCandidateCursor(v.view)
	requireEqual(t, ok, true, "%s", "no first cursor")
	for range 8 {
		cursor, ok = nextCursor(v.view, cursor.pane, cursor.row)
		requireEqual(t, ok, true, "%s", "next cursor missing")
	}
	v.cursor = cursor
	v.viewport = initialViewport(v.view, 30, 5)
	v.viewport = cursorViewport(v.view, v.viewport, cursor)
	viewport := v.viewport
	requireEqual(t, cursor.row >= viewport.top, true,
		"cursor not visible: %#v %#v", cursor, viewport,
	)
	requireEqual(t, cursor.row < viewport.top+viewport.Height, true,
		"cursor not visible: %#v %#v", cursor, viewport,
	)

	v.Align(Center)
	viewport = v.viewport
	headerHeight := 0
	if viewportHasFileHeader(v.view, viewport) {
		headerHeight = 1
	}
	requireEqual(t, headerHeight+v.cursor.row-viewport.top, viewport.Height/2,
		"middle alignment = %#v cursor=%#v header=%d",
		viewport, v.cursor, headerHeight,
	)

	v.ScrollHorizontal(4)
	requireEqual(t, v.viewport.LeftColumn != 0, true, "%s",
		"horizontal scroll did not move",
	)
	v.viewport.Width, v.viewport.Height = 20, 3
	v.viewport = clampViewport(v.view, v.viewport)
	requireEqual(t, v.viewport.Width, 20, "resize = %#v", v.viewport)
	requireEqual(t, v.viewport.Height, 3, "resize = %#v", v.viewport)

	before := v.cursor
	v.Move(NextPage)
	requireEqual(t, v.cursor.row >= before.row, true,
		"half page moved backward: %#v -> %#v", before, v.cursor,
	)
}

func TestDiffscreenViewportProgressUsesVisibleBottom(t *testing.T) {
	v := newDiffscreenTestView(
		diffscreenLongContextPatch(), Options{Dark: true},
	)
	v.Resize(30, 5)
	progress := viewportPercent(v.view, v.viewport)
	if progress <= 0 || progress >= 100 {
		t.Fatalf("initial progress=%d", progress)
	}

	last, ok := lastCursor(v.view)
	requireEqual(t, ok, true, "%s", "no last cursor")
	v.cursor = last
	v.viewport = cursorViewport(v.view, v.viewport, last)
	progress = viewportPercent(v.view, v.viewport)
	if progress != 100 {
		t.Fatalf("final progress=%d", progress)
	}
}

func TestDiffscreenHalfPageScrollingMovesCursorToFileBoundaries(t *testing.T) {
	v := newDiffscreenTestView(
		diffscreenLongContextPatch(), Options{Dark: true},
	)
	first, ok := diffscreenSearchCandidateCursor(v.view)
	requireEqual(t, ok, true, "%s", "no first cursor")
	last, ok := lastCursor(v.view)
	requireEqual(t, ok, true, "%s", "no last cursor")
	v.cursor = first
	v.viewport = initialViewport(v.view, 30, 5)

	steps := len(diffscreenLongContextPatch().Files[0].Hunks[0].Lines)
	for range steps {
		v.Move(NextPage)
	}
	requireEqual(t, v.cursor, last,
		"cursor after scrolling down = %#v, want %#v", v.cursor, last,
	)

	for range steps {
		v.Move(PreviousPage)
	}
	requireEqual(t, v.cursor, first,
		"cursor after scrolling up = %#v, want %#v", v.cursor, first,
	)
}

func TestDiffscreenFindCursorUsesSemanticIdentityAcrossChangedCoordinates(
	t *testing.T,
) {
	original := diffscreenTwoFileSelectionPatch()
	oldView := diffscreenTestViewWithTheme(original, true)
	query := "added one"
	first, ok := diffscreenSearchCandidateCursor(oldView)
	requireEqual(t, ok, true, "%s", "no first cursor")
	cursor, ok := findSearchCursor(oldView, query, first)
	requireEqual(t, ok, true, "%s", "search candidate not found")
	_, line, ok := patchLineAtCursor(oldView, cursor)
	requireEqual(t, ok, true, "%s", "search candidate has no line")
	requireEqual(t, line,
		patches.Line{Kind: patches.Addition, Text: "added one", NewNumber: 2},
		"search line = %#v", line,
	)
	changed := diffscreenTwoFileSelectionPatch()
	changed.Files[0].Metadata = []string{"mode changed", "more metadata"}
	newView := diffscreenTestViewWithTheme(changed, true)
	identity := cursorIdentityAt(oldView, cursor)
	translated, ok := findCursorMatch(newView, identity)
	requireEqual(t, ok, true, "%s", "semantic cursor was not found")
	requireEqual(t, translated.row != cursor.row, true, "%s",
		"cursor coordinate was reused after rows shifted",
	)

	_, translatedLine, _ := patchLineAtCursor(newView, translated)
	requireEqual(t, translatedLine, line,
		"translated line = %#v, want %#v", translatedLine, line,
	)
}

func TestDiffscreenSplitViewWithOnlyDeletionsStartsInLeftPane(t *testing.T) {
	p := patches.Patch{
		Files: []patches.File{
			{
				DisplayPath: "deleted.go",
				Hunks: []patches.Hunk{
					{
						Header: "@@",
						Lines: []patches.Line{
							{
								Kind:      patches.Deletion,
								Text:      "gone",
								OldNumber: 1,
							},
						},
					},
				},
			},
		},
	}
	v := diffscreenTestSplitViewWithTheme(p, true)
	cursor, ok := diffscreenSearchCandidateCursor(v)
	requireEqual(t, ok, true, "first cursor = %#v, %v", cursor, ok)
	requireEqual(t, cursor.pane, left, "first cursor = %#v, %v",
		cursor, ok,
	)

	_, line, _ := patchLineAtCursor(v, cursor)
	requireEqual(t, line.Text, "gone", "first line = %q", line.Text)
}

func TestDiffscreenFindCursorFallsBackNearRemovedLine(t *testing.T) {
	original := diffscreenTwoFileSelectionPatch()
	oldView := diffscreenTestViewWithTheme(original, true)
	query := "removed two"
	first, ok := diffscreenSearchCandidateCursor(oldView)
	requireEqual(t, ok, true, "%s", "no first cursor")
	cursor, ok := findSearchCursor(oldView, query, first)
	requireEqual(t, ok, true, "%s", "search candidate not found")
	_, line, ok := patchLineAtCursor(oldView, cursor)
	requireEqual(t, ok, true, "%s", "search result has no line")
	requireEqual(t, line,
		patches.Line{Kind: patches.Deletion, Text: "removed two", OldNumber: 3},
		"search line = %#v", line,
	)
	changed := diffscreenTwoFileSelectionPatch()
	changed.Files[0].Hunks[0].Lines = changed.Files[0].Hunks[0].Lines[:2]
	newView := diffscreenTestViewWithTheme(changed, true)
	identity := cursorIdentityAt(oldView, cursor)
	fallback, ok := findCursorMatch(newView, identity)
	requireEqual(t, ok, true, "%s", "nearby cursor was not found")

	_, fallbackLine, _ := patchLineAtCursor(newView, fallback)
	requireEqual(
		t,
		fallbackLine.Kind,
		patches.Deletion,
		"fallback kind = %v",
		fallbackLine.Kind,
	)
}
