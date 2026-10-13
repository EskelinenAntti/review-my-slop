package diffscreen

import (
	"fmt"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestDiffMarkersUseTerminalColorsAndCursorFillsWidth(t *testing.T) {
	v := newUnifiedPatchView(twoFileReviewPatch(), true)
	firstCursor := mustFirstCursor(t, v)
	added, _ := v.searchFromCursor("added one", firstCursor, forward)
	removed, _ := v.searchFromCursor("removed one", firstCursor, forward)
	addedRender := v.renderViewportAtRow(added, firstCursor, 80, nil)
	removedRender := v.renderViewportAtRow(removed, firstCursor, 80, nil)
	testutil.RequireEqual(t, strings.Contains(addedRender, "\x1b[32m+\x1b[m"),
		true, fmt.Sprintf("added=%q removed=%q", addedRender, removedRender),
	)
	testutil.RequireEqual(t,
		strings.Contains(removedRender, "\x1b[31m-\x1b[m"), true,
		fmt.Sprintf("added=%q removed=%q", addedRender, removedRender),
	)

	assertStyledThroughColumn(
		t,
		renderFocusedRow(v, added, 80, nil),
		80,
		sgrExpectation{reverse: true},
	)
}

func TestSelectionBackgroundKeepsDefaultWeight(t *testing.T) {
	v := newUnifiedPatchView(twoFileReviewPatch(), false)
	firstCursor := mustFirstCursor(t, v)
	removed, _ := v.searchFromCursor("removed one", firstCursor, forward)
	selection := beginRange(removed)
	rendered := v.renderViewportAtRow(removed, firstCursor, 72, &selection)
	testutil.RequireEqual(t, !strings.Contains(rendered, "\x1b[1m"), true,
		fmt.Sprintf("selection is bold: %q", rendered),
	)

	assertStyledThroughColumn(
		t,
		rendered,
		72,
		sgrExpectation{background: "219;234;254"},
	)
}

func TestSyntaxHighlightingSurvivesDiffStyling(t *testing.T) {
	p := patch.Patch{
		Files: []patch.File{
			{
				DisplayPath: "main.go",
				OldPath:     "main.go",
				NewPath:     "main.go",
				OldSource:   "package main\nold()\n",
				NewSource:   "package main\nnew()\n",
				Hunks: []patch.Hunk{
					{
						Header: "@@",
						Lines: []patch.Line{
							{Kind: patch.Deletion, Text: "old()", OldNumber: 2},
							{Kind: patch.Addition, Text: "new()", NewNumber: 2},
						},
					},
				},
			},
		},
	}
	v := newUnifiedPatchView(p, true)
	firstCursor := mustFirstCursor(t, v)
	added, _ := v.searchFromCursor("new()", firstCursor, forward)
	for _, cursor := range []patchCursor{firstCursor, added} {
		rendered := v.renderViewportAtRow(cursor, patchCursor{}, 80, nil)
		testutil.RequireEqual(t, strings.Contains(rendered, "[38;2;"), true,
			fmt.Sprintf("syntax highlighting missing: %q", rendered),
		)
	}
}

func TestRenderedCodeRowsHaveExactTerminalWidth(t *testing.T) {
	for _, test := range []struct {
		constructor func(patch.Patch, bool) *patchProjection
		width       int
	}{{newUnifiedPatchView, 37}, {newSplitPatchView, 120}} {
		v := test.constructor(twoFileReviewPatch(), true)
		cursor := mustFirstCursor(t, v)
		for {
			row := renderFocusedRow(v, cursor, test.width, nil)
			width := lipgloss.Width(row)
			testutil.RequireEqual(t, width, test.width,
				fmt.Sprintf("width=%d want=%d", width, test.width),
			)
			next, ok := v.moveCursor(cursor, forward)
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
		80,
		preserveForeground,
	)
	for _, forbidden := range []string{
		"48;2;255;0;0",
		"48;5;123",
		"[45m",
		"[105m",
	} {
		testutil.RequireEqual(t, !strings.Contains(rendered, forbidden), true,
			fmt.Sprintf("retains %q: %q", forbidden, rendered),
		)
	}
}
