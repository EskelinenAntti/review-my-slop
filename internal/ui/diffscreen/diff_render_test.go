package diffscreen

import (
	"math"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

type sgrExpectation struct {
	background string
	reverse    bool
}
type sgrState struct {
	background string
	reverse    bool
}

var sgrPattern = regexp.MustCompile(`\x1b\[([0-9;]*)m`)

func TestHorizontalScrollKeepsUnifiedGutterFixed(t *testing.T) {
	p := longLinePatch()
	v := newUnifiedView(p, true)
	cursor := mustFirst(t, v)
	viewport := v.newViewport(37, 1)
	viewport = v.keepVisible(viewport, cursor)
	before := ansi.Strip(v.render(viewport, cursor, nil))
	viewport = v.scrollHorizontal(viewport, 4)
	after := ansi.Strip(v.render(viewport, cursor, nil))
	testutil.RequireEqual(t, before[:14], after[:14],
		"before=%q after=%q viewport=%#v", before, after, viewport,
	)
	testutil.RequireEqual(t, strings.Contains(after[14:], "efghij"), true,
		"before=%q after=%q viewport=%#v", before, after, viewport,
	)
	testutil.RequireEqual(t, viewport.leftColumn, 4,
		"before=%q after=%q viewport=%#v", before, after, viewport,
	)
}

func TestHorizontalScrollKeepsSplitGuttersAndDividerFixed(t *testing.T) {
	v := newSideBySideView(longLinePatch(), true)
	cursor := mustFirst(t, v)
	viewport := v.newViewport(120, 1)
	viewport = v.keepVisible(viewport, cursor)
	viewport = v.scrollHorizontal(viewport, 8)
	rendered := ansi.Strip(v.render(viewport, cursor, nil))
	testutil.RequireEqual(t, strings.Index(rendered, "│"), 59,
		"gutters moved: %q", rendered,
	)
	testutil.RequireEqual(t, rendered[:6], "    1 ", "gutters moved: %q",
		rendered,
	)
	testutil.RequireEqual(t, rendered[63:69], "    1 ", "gutters moved: %q",
		rendered,
	)
	testutil.RequireEqual(t, strings.Contains(rendered[6:59], "ghijkl"), true,
		"offset differs: %q", rendered,
	)
	testutil.RequireEqual(t, strings.Contains(rendered[69:], "ghijkl"), true,
		"offset differs: %q", rendered,
	)
}

func TestHorizontalScrollStartAndEndClamp(t *testing.T) {
	v := newUnifiedView(longLinePatch(), true)
	viewport := v.newViewport(37, 1)
	viewport = v.scrollHorizontal(viewport, math.MaxInt)
	testutil.RequireEqual(t, viewport.leftColumn != 0, true, "%s",
		"end did not scroll",
	)

	viewport = v.scrollHorizontal(viewport, -math.MaxInt)
	testutil.RequireEqual(t, viewport.leftColumn, 0, "start = %d",
		viewport.leftColumn,
	)
}

func TestDiffMarkersUseTerminalColorsAndCursorFillsWidth(t *testing.T) {
	v := newUnifiedView(testPatch(), true)
	first := mustFirst(t, v)
	added, _ := v.search("added one", first, forward)
	removed, _ := v.search("removed one", first, forward)
	addedRender := renderTarget(v, added, first, 80, nil)
	removedRender := renderTarget(v, removed, first, 80, nil)
	testutil.RequireEqual(t, strings.Contains(addedRender, "\x1b[32m+\x1b[m"),
		true, "added=%q removed=%q", addedRender, removedRender,
	)
	testutil.RequireEqual(t, strings.Contains(removedRender, "\x1b[31m-\x1b[m"),
		true, "added=%q removed=%q", addedRender, removedRender,
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
	rendered := renderTarget(v, removed, first, 72, &selection)
	testutil.RequireEqual(t, !strings.Contains(rendered, "\x1b[1m"), true,
		"selection is bold: %q", rendered,
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
		rendered := renderTarget(v, cursor, diffCursor{}, 80, nil)
		testutil.RequireEqual(t, strings.Contains(rendered, "[38;2;"), true,
			"syntax highlighting missing: %q", rendered,
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
			testutil.RequireEqual(t, width, test.width, "width=%d want=%d",
				width, test.width,
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
		lineStyle(patch.Addition, newLineStyles(true)),
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
			"retains %q: %q", forbidden, rendered,
		)
	}
}

func renderOne(
	v *diffView,
	cursor diffCursor,
	width int,
	selection *diffSelection,
) string {
	return renderTarget(v, cursor, cursor, width, selection)
}

func renderTarget(
	v *diffView,
	target, active diffCursor,
	width int,
	selection *diffSelection,
) string {
	viewport := v.newViewport(width, 1)
	viewport.top = target.row
	return v.render(viewport, active, selection)
}

func longLinePatch() patch.Patch {
	return patch.Patch{
		Files: []patch.File{
			{
				DisplayPath: "long",
				Hunks: []patch.Hunk{
					{
						Header: "@@",
						Lines: []patch.Line{
							{
								Kind: patch.Context,
								Text: "abcdefghijklmnopqrstuvwxyz" +
									"0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ",
								OldNumber: 1,
								NewNumber: 1,
							},
						},
					},
				},
			},
		},
	}
}

func assertStyledThroughColumn(
	t *testing.T,
	rendered string,
	width int,
	expected sgrExpectation,
) {
	t.Helper()
	state := sgrState{}
	column := 0
	for len(rendered) > 0 {
		location := sgrPattern.FindStringSubmatchIndex(rendered)
		if location == nil {
			column = assertStyledSpan(t, rendered, column, state, expected)
			break
		}
		column = assertStyledSpan(
			t,
			rendered[:location[0]],
			column,
			state,
			expected,
		)
		state.apply(rendered[location[2]:location[3]])
		rendered = rendered[location[1]:]
	}
	testutil.RequireEqual(t, column, width, "styled columns = %d, want %d",
		column, width,
	)
}

func assertStyledSpan(
	t *testing.T,
	text string,
	column int,
	state sgrState,
	expected sgrExpectation,
) int {
	t.Helper()
	for range text {
		column++
		assertSGRState(t, column, state, expected)
	}
	return column
}

func (s *sgrState) apply(parameters string) {
	if parameters == "" {
		parameters = "0"
	}
	values := strings.Split(parameters, ";")
	for index := 0; index < len(values); index++ {
		value, _ := strconv.Atoi(values[index])
		switch value {
		case 0:
			*s = sgrState{}
		case 7:
			s.reverse = true
		case 27:
			s.reverse = false
		case 49:
			s.background = ""
		case 48:
			index = s.applyBackground(values, index)
		default:
			continue
		}
	}
}

func (s *sgrState) applyBackground(values []string, index int) int {
	if index+4 < len(values) && values[index+1] == "2" {
		s.background = strings.Join(values[index+2:index+5], ";")
		return index + 4
	}
	return index
}

func assertSGRState(
	t *testing.T,
	column int,
	state sgrState,
	expected sgrExpectation,
) {
	t.Helper()
	testutil.RequireEqual(t, state.background, expected.background,
		"column %d state=%#v want=%#v", column, state, expected,
	)
	testutil.RequireEqual(t, state.reverse, expected.reverse,
		"column %d state=%#v want=%#v", column, state, expected,
	)
}
