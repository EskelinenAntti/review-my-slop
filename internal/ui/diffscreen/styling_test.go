package diffscreen

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

func TestDiffMarkersUseTerminalColorsAndCursorFillsWidth(t *testing.T) {
	v := newUnifiedView(testPatch(), true)
	first := mustFirst(t, v)
	added, _ := v.search("added one", first, forward)
	removed, _ := v.search(fixtureRemovedOne, first, forward)
	addedRender := renderTarget(v, added, first, defaultWidth, nil)
	removedRender := renderTarget(v, removed, first, defaultWidth, nil)
	requireEqual(
		t,
		strings.Contains(addedRender, "\x1b[32m+\x1b[m") &&
			strings.Contains(removedRender, "\x1b[31m-\x1b[m"),
		true,
		"added=%q removed=%q",
		addedRender,
		removedRender,
	)
	assertStyledThroughColumn(
		t,
		renderOne(v, added, defaultWidth, nil),
		defaultWidth,
		sgrExpectation{reverse: true},
	)
}

func TestSelectionBackgroundKeepsDefaultWeight(t *testing.T) {
	v := newUnifiedView(testPatch(), false)
	first := mustFirst(t, v)
	removed, _ := v.search(fixtureRemovedOne, first, forward)
	selection := v.beginSelection(removed)
	rendered := renderTarget(v, removed, first, renderTestWidth, &selection)
	requireEqual(
		t,
		!strings.Contains(rendered, "\x1b[1m"), true,
		"selection is bold: %q",
		rendered,
	)
	assertStyledThroughColumn(
		t,
		rendered,
		renderTestWidth,
		sgrExpectation{background: "219;234;254"},
	)
}

func TestSyntaxHighlightingSurvivesDiffStyling(t *testing.T) {
	p := patch.Patch{
		Files: []patch.File{
			{
				DisplayPath: fixtureMainGo,
				OldPath:     fixtureMainGo,
				NewPath:     fixtureMainGo,
				OldSource:   "package main\nold()\n",
				NewSource:   "package main\nnew()\n",
				Hunks: []patch.Hunk{
					{
						Header: hunkMarker,
						Lines: []patch.Line{
							{
								Kind:      patch.Deletion,
								Text:      "old()",
								OldNumber: 2,
							},
							{
								Kind:      patch.Addition,
								Text:      "new()",
								NewNumber: 2,
							},
						},
					},
				},
			},
		},
	}
	v := newUnifiedView(p, true)
	first := mustFirst(t, v)
	added, _ := v.search("new()", first, forward)
	for _, cursor := range []diffCursor{first, added} {
		rendered := renderTarget(v, cursor, diffCursor{}, defaultWidth, nil)
		requireEqual(
			t,
			strings.Contains(rendered, "[38;2;"), true,
			"syntax highlighting missing: %q",
			rendered,
		)
	}
}

func TestRenderedCodeRowsHaveExactTerminalWidth(t *testing.T) {
	for _, test := range []struct {
		constructor func(patch.Patch, bool) *diffView
		width       int
	}{{newUnifiedView, narrowTestWidth}, {newSideBySideView, wideTestWidth}} {
		v := test.constructor(testPatch(), true)
		cursor := mustFirst(t, v)
		for {
			{
				width := lipgloss.Width(renderOne(v, cursor, test.width, nil))
				requireEqual(t, width, test.width, "width = %d", width)
			}
			next, ok := v.move(cursor, forward)
			if !ok {
				break
			}
			cursor = next
		}
	}
}

func TestRenderStyledRowStripsSyntaxBackgroundColors(t *testing.T) {
	value := strings.Join(
		[]string{
			"\x1b[48;2;255;0;0;38;2;1;2;3mtruecolor",
			"\x1b[48;5;123;1mindexed",
			"\x1b[45mstandard",
			"\x1b[105mbright",
		},
		" ",
	)
	rendered := renderStyledRow(
		lineStyle(patch.Addition, true),
		value,
		defaultWidth,
		false,
	)
	for _, forbidden := range []string{
		"48;2;255;0;0",
		"48;5;123",
		"[45m",
		"[105m",
	} {
		requireEqual(
			t,
			!strings.Contains(rendered, forbidden), true,
			"retains %q: %q",
			forbidden,
			rendered,
		)
	}
}
