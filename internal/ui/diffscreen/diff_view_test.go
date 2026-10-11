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
	requireEqual(t, ok, true, fixtureV, "First returned no cursor")
	line, _ := v.line(first)
	requireEqual(t, line.Text, "before", "first line = %q", line.Text)
	next, ok := v.move(first, forward)
	requireEqual(t, ok, true, fixtureV, "Move returned no cursor")
	line, _ = v.line(next)
	requireEqual(t, line.Kind, patch.Deletion, "next kind = %v", line.Kind)
	match, ok := v.search("added", first, forward)
	requireEqual(t, ok, true, fixtureV, "Search returned no cursor")
	line, _ = v.line(match)
	requireEqual(t, line.Text, fixtureAddedOne, "match = %q", line.Text)
	jumped, ok := v.jumpFile(first, forward)
	requireEqual(t, ok, true, fixtureV, "JumpFile returned no cursor")
	file, _ := v.file(jumped)
	requireEqual(
		t,
		file.DisplayPath, fixtureSecondGo,
		"jumped file = %q",
		file.DisplayPath,
	)
	{
		_, ok := v.jumpFile(jumped, forward)
		requireEqual(t, !ok, true, fixtureV, "JumpFile wrapped unexpectedly")
	}
}

func TestSplitPairsChangeBlocksAndSupportsEmptyPanes(t *testing.T) {
	v := newSideBySideView(testPatch(), true)
	first, _ := v.first()
	removed, _ := v.move(first, forward)
	requireEqual(t, removed.pane, right, "initial pane = %v", removed.pane)
	removed, ok := v.switchPane(removed, left)
	requireEqual(t, ok, true, fixtureV, "could not switch to deletion pane")
	line, _ := v.line(removed)
	requireEqual(t, line.Text, "removed one", "left line = %q", line.Text)
	added, ok := v.switchPane(removed, right)
	requireEqual(t, ok, true, fixtureV, "paired addition missing")
	line, _ = v.line(added)
	requireEqual(t, line.Text, fixtureAddedOne, "right line = %q", line.Text)
	secondRemoved, _ := v.move(removed, forward)
	{
		_, ok := v.switchPane(secondRemoved, right)
		requireEqual(
			t,
			ok,
			true,
			fixtureV,
			"pane switch should find a nearby right line",
		)
	}

	viewport := v.newViewport(hundred, minimumWidth)
	rendered := v.render(viewport, added, nil)
	requireEqual(
		t,
		strings.Contains(rendered, "removed one") &&
			strings.Contains(rendered, fixtureAddedOne),
		true,
		"paired render missing lines: %q",
		rendered,
	)
	for _, row := range strings.Split(rendered, "\n") {
		requireEqual(
			t,
			!strings.Contains(row, " │ ") || lipgloss.Width(row) == hundred,
			true,
			"rendered width = %d",
			lipgloss.Width(row),
		)
	}
}

func TestSelectionLines(t *testing.T) {
	v := newUnifiedView(testPatch(), true)
	first, _ := v.first()
	last, _ := v.move(first, forward)
	last, _ = v.move(last, forward)
	selection := v.beginSelection(first)
	selection, ok := v.extendSelection(selection, last)
	requireEqual(t, ok, true, fixtureV, "selection extension failed")
	lines := v.lines(selection)
	requireEqual(t, len(lines), three, "selected lines = %d", len(lines))

	nextFile, _ := v.jumpFile(first, forward)
	{
		_, ok := v.extendSelection(selection, nextFile)
		requireEqual(t, !ok, true, fixtureV, "selection crossed a hunk")
	}
}

func TestViewportAlignmentResizeAndScrolling(t *testing.T) {
	v := newUnifiedView(longPatch(), true)
	first, _ := v.first()
	cursor := first
	for range eight {
		cursor, _ = v.move(cursor, forward)
	}
	viewport := v.newViewport(defaultHeight, five)
	viewport = v.keepVisible(viewport, cursor)
	requireEqual(
		t,
		cursor.row >= viewport.top &&
			cursor.row < viewport.top+viewport.Height,
		true,
		"cursor not visible: %#v %#v",
		cursor,
		viewport,
	)
	viewport = v.align(viewport, cursor, middle)
	headerHeight := zero
	if v.hasStickyHeader(viewport.top, viewport.Height) {
		headerHeight = one
	}
	requireEqual(
		t,
		headerHeight+cursor.row-viewport.top, viewport.Height/two,
		"middle alignment = %#v",
		viewport,
	)
	viewport = v.scrollHorizontal(viewport, four)
	requireEqual(
		t,
		viewport.LeftColumn != zero,
		true,
		fixtureV,
		"horizontal scroll did not move",
	)
	viewport = v.resize(viewport, minimumWidth, three)
	requireEqual(
		t,
		viewport.Width == minimumWidth && viewport.Height == three, true,
		"resize = %#v",
		viewport,
	)
	before := cursor
	viewport, cursor = v.scrollHalfPage(viewport, cursor, forward)
	requireEqual(
		t,
		cursor.row >= before.row, true,
		"half page moved backward: %#v -> %#v",
		before,
		cursor,
	)
}

func TestViewportProgressUsesVisibleBottom(t *testing.T) {
	v := newUnifiedView(longPatch(), true)
	viewport := v.newViewport(defaultHeight, five)
	{
		progress := v.viewportProgress(viewport)
		requireEqual(
			t,
			progress > zero && progress < hundred,
			true,
			"initial progress=%d",
			progress,
		)
	}
	last, _ := v.last()
	viewport = v.keepVisible(viewport, last)
	{
		progress := v.viewportProgress(viewport)
		requireEqual(t, progress, hundred, "final progress=%d", progress)
	}
}

func TestHalfPageScrollingMovesCursorToFileBoundaries(t *testing.T) {
	v := newUnifiedView(longPatch(), true)
	first, _ := v.first()
	last, _ := v.last()
	viewport := v.newViewport(defaultHeight, five)
	cursor := first

	for range len(longPatch().Files[zero].Hunks[zero].Lines) {
		viewport, cursor = v.scrollHalfPage(viewport, cursor, forward)
	}
	requireEqual(
		t,
		cursor, last,
		"cursor after scrolling down = %#v, want %#v",
		cursor,
		last,
	)

	for range len(longPatch().Files[zero].Hunks[zero].Lines) {
		viewport, cursor = v.scrollHalfPage(viewport, cursor, backward)
	}
	requireEqual(
		t,
		cursor, first,
		"cursor after scrolling up = %#v, want %#v",
		cursor,
		first,
	)
}

func TestFindCursorUsesSemanticIdentityAcrossChangedCoordinates(t *testing.T) {
	original := testPatch()
	oldView := newUnifiedView(original, true)
	cursor, _ := oldView.search(
		fixtureAddedOne,
		mustFirst(t, oldView),
		forward,
	)
	line, _ := oldView.line(cursor)
	changed := testPatch()
	changed.Files[zero].Metadata = []string{"mode changed", "more metadata"}
	newView := newUnifiedView(changed, true)
	translated, ok := newView.findCursor(identify(oldView, cursor))
	requireEqual(t, ok, true, fixtureV, "semantic cursor was not found")
	requireEqual(
		t,
		translated.row != cursor.row,
		true,
		fixtureV,
		"cursor coordinate was reused after rows shifted",
	)
	translatedLine, _ := newView.line(translated)
	requireEqual(
		t,
		translatedLine, line,
		"translated line = %#v, want %#v",
		translatedLine,
		line,
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
							{
								Kind:      patch.Deletion,
								Text:      "gone",
								OldNumber: one,
							},
						},
					},
				},
			},
		},
	}
	v := newSideBySideView(p, true)
	cursor, ok := v.first()
	requireEqual(
		t,
		ok && cursor.pane == left, true,
		"first cursor = %#v, %v",
		cursor,
		ok,
	)
	line, _ := v.line(cursor)
	requireEqual(t, line.Text, "gone", "first line = %q", line.Text)
}

func TestFindCursorFallsBackNearRemovedLine(t *testing.T) {
	original := testPatch()
	oldView := newUnifiedView(original, true)
	cursor, _ := oldView.search("removed two", mustFirst(t, oldView), forward)
	changed := testPatch()
	hunk := &changed.Files[zero].Hunks[zero]
	hunk.Lines = hunk.Lines[:two]
	newView := newUnifiedView(changed, true)
	fallback, ok := newView.findCursor(identify(oldView, cursor))
	requireEqual(t, ok, true, fixtureV, "nearby cursor was not found")
	fallbackLine, _ := newView.line(fallback)
	requireEqual(
		t,
		fallbackLine.Kind, patch.Deletion,
		"fallback kind = %v",
		fallbackLine.Kind,
	)
}

func mustFirst(t *testing.T, v *diffView) diffCursor {
	t.Helper()
	cursor, ok := v.first()
	requireEqual(t, ok, true, fixtureV, "no first cursor")
	return cursor
}

func testPatch() patch.Patch {
	return patch.Patch{
		Root: "/repo",

		Files: []patch.File{
			{
				DisplayPath: fixtureFirstGo,

				OldPath: fixtureFirstGo,

				NewPath: fixtureFirstGo,

				Hunks: []patch.Hunk{
					{
						Header: "@@ -1,3 +1,3 @@",

						Lines: []patch.Line{
							{
								Kind:      patch.Context,
								Text:      "before",
								OldNumber: one,
								NewNumber: one,
							},

							{
								Kind:      patch.Deletion,
								Text:      "removed one",
								OldNumber: two,
							},

							{
								Kind:      patch.Deletion,
								Text:      "removed two",
								OldNumber: three,
							},

							{
								Kind:      patch.Addition,
								Text:      fixtureAddedOne,
								NewNumber: two,
							},

							{
								Kind:      patch.Context,
								Text:      "after",
								OldNumber: four,
								NewNumber: three,
							},
						},
					},
				},
			},

			{
				DisplayPath: fixtureSecondGo,
				OldPath:     fixtureSecondGo,
				NewPath:     fixtureSecondGo,
				Hunks: []patch.Hunk{
					{
						Header: "@@ -1 +1 @@",
						Lines: []patch.Line{
							{
								Kind:      patch.Addition,
								Text:      "other",
								NewNumber: one,
							},
						},
					},
				},
			},
		},
	}
}

func longPatch() patch.Patch {
	lines := make([]patch.Line, minimumWidth)
	for index := range lines {
		lines[index] = patch.Line{
			Kind:      patch.Context,
			Text:      strings.Repeat("long", minimumWidth),
			OldNumber: patch.LineNumber(index + one),
			NewNumber: patch.LineNumber(index + one),
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
	return newDiffView(p, Options{SideBySide: false, Dark: dark})
}

func newSideBySideView(p patch.Patch, dark bool) *diffView {
	return newDiffView(p, Options{SideBySide: true, Dark: dark})
}
