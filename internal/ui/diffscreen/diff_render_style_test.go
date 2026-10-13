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
	v := newUnifiedView(testPatch(), true)
	first := mustFirst(t, v)
	added, _ := v.search("added one", first, forward)
	removed, _ := v.search("removed one", first, forward)
	addedRender := v.renderTarget(added, first, 80, nil)
	removedRender := v.renderTarget(removed, first, 80, nil)
	testutil.RequireEqual(t, strings.Contains(addedRender, "\x1b[32m+\x1b[m"),
		true, fmt.Sprintf("added=%q removed=%q", addedRender, removedRender),
	)
	testutil.RequireEqual(t,
		strings.Contains(removedRender, "\x1b[31m-\x1b[m"), true,
		fmt.Sprintf("added=%q removed=%q", addedRender, removedRender),
	)

	assertStyledThroughColumn(
		t,
		renderOne(v, added, 80, nil),
		80,
		sgrExpectation{reverse: true},
	)
}

func TestSelectionBackgroundKeepsDefaultWeight(t *testing.T) {
	v := newUnifiedView(testPatch(), false)
	first := mustFirst(t, v)
	removed, _ := v.search("removed one", first, forward)
	selection := beginSelection(removed)
	rendered := v.renderTarget(removed, first, 72, &selection)
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
	v := newUnifiedView(p, true)
	first := mustFirst(t, v)
	added, _ := v.search("new()", first, forward)
	for _, cursor := range []diffCursor{first, added} {
		rendered := v.renderTarget(cursor, diffCursor{}, 80, nil)
		testutil.RequireEqual(t, strings.Contains(rendered, "[38;2;"), true,
			fmt.Sprintf("syntax highlighting missing: %q", rendered),
		)
	}
}

func TestRenderedCodeRowsHaveExactTerminalWidth(t *testing.T) {
	for _, test := range []struct {
		constructor func(patch.Patch, bool) *diffView
		width       int
	}{{newUnifiedView, 37}, {newSideBySideView, 120}} {
		v := test.constructor(testPatch(), true)
		cursor := mustFirst(t, v)
		for {
			width := lipgloss.Width(renderOne(v, cursor, test.width, nil))
			testutil.RequireEqual(t, width, test.width,
				fmt.Sprintf("width=%d want=%d", width, test.width),
			)
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
