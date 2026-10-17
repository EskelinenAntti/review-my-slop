package main

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

func TestDiffscreenUnifiedNavigationSearchAndFileJumps(t *testing.T) {
	v := diffscreenTestViewWithTheme(diffscreenTwoFileSelectionPatch(), true)
	first, ok := diffscreenSearchCandidateCursor(v)
	requireEqual(t, ok, true, "%s", "First returned no cursor")

	_, line, _ := diffscreenPatchLineAtCursor(v, first)
	requireEqual(t, line.Text, "before", "first line = %q", line.Text)

	next, ok := diffscreenScanCursor(v, first, diffscreenPrivateForward)
	requireEqual(t, ok, true, "%s", "Move returned no cursor")

	_, line, _ = diffscreenPatchLineAtCursor(v, next)
	requireEqual(t, line.Kind, patchDeletion, "next kind = %v", line.Kind)

	match, ok := diffscreenFindSearchCursor(v, "added", first)
	requireEqual(t, ok, true, "%s", "Search returned no cursor")

	_, line, _ = diffscreenPatchLineAtCursor(v, match)
	requireEqual(t, line.Text, "added one", "match = %q", line.Text)

	jumped, ok := diffscreenJumpFile(v, first, diffscreenPrivateForward)
	requireEqual(t, ok, true, "%s", "JumpFile returned no cursor")

	file, _, _ := diffscreenPatchLineAtCursor(v, jumped)
	requireEqual(t, file.DisplayPath, "second.go", "jumped file = %q",
		file.DisplayPath,
	)

	_, wrapped := diffscreenNextCursor(v, jumped.pane, jumped.row)
	requireEqual(t, wrapped, false, "%s", "JumpFile wrapped unexpectedly")
}

func TestDiffscreenSplitPairsChangeBlocksAndSupportsEmptyPanes(t *testing.T) {
	v := diffscreenTestSplitViewWithTheme(
		diffscreenTwoFileSelectionPatch(), true,
	)
	first, ok := diffscreenSearchCandidateCursor(v)
	requireEqual(t, ok, true, "%s", "no first cursor")
	removed, ok := diffscreenNextCursor(v, first.pane, first.row)
	requireEqual(t, ok, true, "%s", "no next cursor")
	requireEqual(t, removed.pane, diffscreenRight,
		"initial pane = %v", removed.pane,
	)

	removed, ok = diffscreenFindCursorInPane(diffscreenLeft, v, removed)
	requireEqual(t, ok, true, "%s", "could not switch to deletion pane")
	_, line, _ := diffscreenPatchLineAtCursor(v, removed)
	requireEqual(t, line.Text, "removed one", "left line = %q", line.Text)

	added, ok := diffscreenFindCursorInPane(diffscreenRight, v, removed)
	requireEqual(t, ok, true, "%s", "paired addition missing")
	_, line, _ = diffscreenPatchLineAtCursor(v, added)
	requireEqual(t, line.Text, "added one", "right line = %q", line.Text)

	secondRemoved, ok := diffscreenNextCursor(v, removed.pane, removed.row)
	requireEqual(t, ok, true, "%s", "second deletion missing")
	_, ok = diffscreenFindCursorInPane(diffscreenRight, v, secondRemoved)
	requireEqual(t, ok, true, "%s",
		"pane switch should find a nearby right line",
	)

	viewport := diffscreenInitialViewport(v, 100, 20)
	rendered := diffscreenRenderPatch(v, viewport, added, nil)
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
	last, ok := diffscreenNextCursor(v, first.pane, first.row)
	requireEqual(t, ok, true, "%s", "no second cursor")
	last, ok = diffscreenNextCursor(v, last.pane, last.row)
	requireEqual(t, ok, true, "%s", "no third cursor")

	selection, ok := diffscreenSelectionForDrag(v, first, last)
	requireEqual(t, ok, true, "%s", "selection extension failed")
	lines := diffscreenLinesInSelection(v, selection)
	requireEqual(t, len(lines), 3, "selected lines = %d", len(lines))

	nextFile, ok := diffscreenJumpFile(v, first, diffscreenPrivateForward)
	requireEqual(t, ok, true, "%s", "next file cursor missing")
	_, ok = diffscreenSelectionForDrag(v, first, nextFile)
	requireEqual(t, ok, false, "%s", "selection crossed a hunk")
}

func TestDiffscreenViewportAlignmentResizeAndScrolling(t *testing.T) {
	v := newDiffscreenTestView(
		diffscreenLongContextPatch(), diffscreenOptions{Dark: true},
	)
	cursor, ok := diffscreenSearchCandidateCursor(v.view)
	requireEqual(t, ok, true, "%s", "no first cursor")
	for range 8 {
		cursor, ok = diffscreenNextCursor(v.view, cursor.pane, cursor.row)
		requireEqual(t, ok, true, "%s", "next cursor missing")
	}
	v.cursor = cursor
	v.viewport = diffscreenInitialViewport(v.view, 30, 5)
	v.viewport = diffscreenCursorViewport(v.view, v.viewport, cursor)
	viewport := v.viewport
	requireEqual(t, cursor.row >= viewport.top, true,
		"cursor not visible: %#v %#v", cursor, viewport,
	)
	requireEqual(t, cursor.row < viewport.top+viewport.Height, true,
		"cursor not visible: %#v %#v", cursor, viewport,
	)

	v.Align(diffscreenCenter)
	viewport = v.viewport
	headerHeight := 0
	if diffscreenViewportHasFileHeader(v.view, viewport) {
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
	v.viewport = diffscreenClampViewport(v.view, v.viewport)
	requireEqual(t, v.viewport.Width, 20, "resize = %#v", v.viewport)
	requireEqual(t, v.viewport.Height, 3, "resize = %#v", v.viewport)

	before := v.cursor
	v.Move(diffscreenNextPage)
	requireEqual(t, v.cursor.row >= before.row, true,
		"half page moved backward: %#v -> %#v", before, v.cursor,
	)
}

func TestDiffscreenViewportProgressUsesVisibleBottom(t *testing.T) {
	v := newDiffscreenTestView(
		diffscreenLongContextPatch(), diffscreenOptions{Dark: true},
	)
	v.Resize(30, 5)
	progress := diffscreenViewportPercent(v.view, v.viewport)
	if progress <= 0 || progress >= 100 {
		t.Fatalf("initial progress=%d", progress)
	}

	last, ok := diffscreenLastCursor(v.view)
	requireEqual(t, ok, true, "%s", "no last cursor")
	v.cursor = last
	v.viewport = diffscreenCursorViewport(v.view, v.viewport, last)
	progress = diffscreenViewportPercent(v.view, v.viewport)
	if progress != 100 {
		t.Fatalf("final progress=%d", progress)
	}
}

func TestDiffscreenHalfPageScrollingMovesCursorToFileBoundaries(t *testing.T) {
	v := newDiffscreenTestView(
		diffscreenLongContextPatch(), diffscreenOptions{Dark: true},
	)
	first, ok := diffscreenSearchCandidateCursor(v.view)
	requireEqual(t, ok, true, "%s", "no first cursor")
	last, ok := diffscreenLastCursor(v.view)
	requireEqual(t, ok, true, "%s", "no last cursor")
	v.cursor = first
	v.viewport = diffscreenInitialViewport(v.view, 30, 5)

	steps := len(diffscreenLongContextPatch().Files[0].Hunks[0].Lines)
	for range steps {
		v.Move(diffscreenNextPage)
	}
	requireEqual(t, v.cursor, last,
		"cursor after scrolling down = %#v, want %#v", v.cursor, last,
	)

	for range steps {
		v.Move(diffscreenPreviousPage)
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
	cursor, ok := diffscreenFindSearchCursor(oldView, query, first)
	requireEqual(t, ok, true, "%s", "search candidate not found")
	_, line, ok := diffscreenPatchLineAtCursor(oldView, cursor)
	requireEqual(t, ok, true, "%s", "search candidate has no line")
	requireEqual(t, line,
		patchLine{Kind: patchAddition, Text: "added one", NewNumber: 2},
		"search line = %#v", line,
	)
	changed := diffscreenTwoFileSelectionPatch()
	changed.Files[0].Metadata = []string{"mode changed", "more metadata"}
	newView := diffscreenTestViewWithTheme(changed, true)
	identity := diffscreenCursorIdentityAt(oldView, cursor)
	translated, ok := diffscreenTranslateCursorIdentity(newView, identity)
	requireEqual(t, ok, true, "%s", "semantic cursor was not found")
	requireEqual(t, translated.row != cursor.row, true, "%s",
		"cursor coordinate was reused after rows shifted",
	)

	_, translatedLine, _ := diffscreenPatchLineAtCursor(newView, translated)
	requireEqual(t, translatedLine, line,
		"translated line = %#v, want %#v", translatedLine, line,
	)
}

func TestDiffscreenSplitViewWithOnlyDeletionsStartsInLeftPane(t *testing.T) {
	p := patchPatch{
		Files: []patchFile{
			{
				DisplayPath: "deleted.go",
				Hunks: []patchHunk{
					{
						Header: "@@",
						Lines: []patchLine{
							{Kind: patchDeletion, Text: "gone", OldNumber: 1},
						},
					},
				},
			},
		},
	}
	v := diffscreenTestSplitViewWithTheme(p, true)
	cursor, ok := diffscreenSearchCandidateCursor(v)
	requireEqual(t, ok, true, "first cursor = %#v, %v", cursor, ok)
	requireEqual(t, cursor.pane, diffscreenLeft, "first cursor = %#v, %v",
		cursor, ok,
	)

	_, line, _ := diffscreenPatchLineAtCursor(v, cursor)
	requireEqual(t, line.Text, "gone", "first line = %q", line.Text)
}

func TestDiffscreenFindCursorFallsBackNearRemovedLine(t *testing.T) {
	original := diffscreenTwoFileSelectionPatch()
	oldView := diffscreenTestViewWithTheme(original, true)
	query := "removed two"
	first, ok := diffscreenSearchCandidateCursor(oldView)
	requireEqual(t, ok, true, "%s", "no first cursor")
	cursor, ok := diffscreenFindSearchCursor(oldView, query, first)
	requireEqual(t, ok, true, "%s", "search candidate not found")
	_, line, ok := diffscreenPatchLineAtCursor(oldView, cursor)
	requireEqual(t, ok, true, "%s", "search result has no line")
	requireEqual(t, line,
		patchLine{Kind: patchDeletion, Text: "removed two", OldNumber: 3},
		"search line = %#v", line,
	)
	changed := diffscreenTwoFileSelectionPatch()
	changed.Files[0].Hunks[0].Lines = changed.Files[0].Hunks[0].Lines[:2]
	newView := diffscreenTestViewWithTheme(changed, true)
	identity := diffscreenCursorIdentityAt(oldView, cursor)
	fallback, ok := diffscreenTranslateCursorIdentity(newView, identity)
	requireEqual(t, ok, true, "%s", "nearby cursor was not found")

	_, fallbackLine, _ := diffscreenPatchLineAtCursor(newView, fallback)
	requireEqual(
		t,
		fallbackLine.Kind,
		patchDeletion,
		"fallback kind = %v",
		fallbackLine.Kind,
	)
}
