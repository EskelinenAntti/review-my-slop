package diffscreen

import (
	"regexp"
	"strconv"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestSplitPairsUnequalChangeBlocksAndKeepsHunksSeparate(t *testing.T) {
	p := patch.Patch{Files: []patch.File{{DisplayPath: "file", Hunks: []patch.Hunk{
		{Header: "one", Lines: []patch.Line{{Kind: patch.Deletion, Text: "d1", OldNumber: 1}, {Kind: patch.Deletion, Text: "d2", OldNumber: 2}, {Kind: patch.Addition, Text: "a1", NewNumber: 1}, {Kind: patch.Addition, Text: "a2", NewNumber: 2}, {Kind: patch.Addition, Text: "a3", NewNumber: 3}, {Kind: patch.Context, Text: "c", OldNumber: 3, NewNumber: 4}}},
		{Header: "two", Lines: []patch.Line{{Kind: patch.Addition, Text: "separate", NewNumber: 5}}},
	}}}}
	v := newSideBySideView(p, true)
	code := codeEntries(v)
	testutil.Requiref(t, len(code) == 5, "visual code entries = %d", len(code))
	testutil.Requiref(t, code[0].leftLine == 0 && code[0].rightLine == 2 && code[1].leftLine == 1 && code[1].rightLine == 3 && code[2].leftLine == -1 && code[2].rightLine == 4, "pairing = %#v", code[:3])
	testutil.Requiref(t, code[4].hunk != code[3].hunk && code[4].leftLine == -1, "hunks were paired: %#v", code[4])
}

func TestSplitSelectionOnlyIncludesActivePane(t *testing.T) {
	v := newSideBySideView(testPatch(), true)
	first := mustFirst(t, v)
	removed, _ := v.search("removed one", first, forward)
	added, _ := v.search("added one", first, forward)
	left := v.beginSelection(removed)
	left, ok := v.extendSelection(left, removed)
	testutil.Requiref(t, ok && len(v.lines(left)) == 1 && v.lines(left)[0].Kind == patch.Deletion, "left lines = %#v", v.lines(left))
	right := v.beginSelection(added)
	testutil.Requiref(t, len(v.lines(right)) == 1 && v.lines(right)[0].Kind == patch.Addition, "right lines = %#v", v.lines(right))
}

func TestSplitPaneSwitchingFindsRowsAboveAndBelowEmptyTargets(t *testing.T) {
	p := patch.Patch{Files: []patch.File{{DisplayPath: "file", Hunks: []patch.Hunk{{Header: "@@", Lines: []patch.Line{
		{Kind: patch.Addition, Text: "right", NewNumber: 1},
		{Kind: patch.Context, Text: "both", OldNumber: 1, NewNumber: 2},
		{Kind: patch.Deletion, Text: "left", OldNumber: 2},
	}}}}}}
	v := newSideBySideView(p, true)
	first := mustFirst(t, v)
	left, ok := v.switchPane(first, left)
	testutil.Require(t, ok, "did not find later left pane")
	line, _ := v.line(left)
	testutil.Requiref(t, line.Text == "both", "later left line = %q", line.Text)
	last, _ := v.last()
	right, ok := v.switchPane(last, right)
	testutil.Require(t, ok, "did not find earlier right pane")
	line, _ = v.line(right)
	testutil.Requiref(t, line.Text == "both", "earlier right line = %q", line.Text)
}

func TestSplitPaneSwitchDoesNothingWhenTargetPaneIsEmpty(t *testing.T) {
	p := patch.Patch{Files: []patch.File{{DisplayPath: "file", Hunks: []patch.Hunk{{Header: "@@", Lines: []patch.Line{{Kind: patch.Addition, Text: "one", NewNumber: 1}, {Kind: patch.Addition, Text: "two", NewNumber: 2}}}}}}}
	v := newSideBySideView(p, true)
	cursor := mustFirst(t, v)
	{
		_, ok := v.switchPane(cursor, left)
		testutil.Require(t, !ok, "switched to empty pane")
	}
}

func TestSplitVerticalMovementSkipsEmptyActivePane(t *testing.T) {
	p := patch.Patch{Files: []patch.File{{DisplayPath: "file", Hunks: []patch.Hunk{{Header: "@@", Lines: []patch.Line{{Kind: patch.Context, Text: "one", OldNumber: 1, NewNumber: 1}, {Kind: patch.Addition, Text: "right", NewNumber: 2}, {Kind: patch.Context, Text: "two", OldNumber: 2, NewNumber: 3}, {Kind: patch.Deletion, Text: "left", OldNumber: 3}, {Kind: patch.Context, Text: "three", OldNumber: 4, NewNumber: 4}}}}}}}
	v := newSideBySideView(p, true)
	first := mustFirst(t, v)
	left, _ := v.switchPane(first, left)
	nextLeft, _ := v.move(left, forward)
	line, _ := v.line(nextLeft)
	testutil.Requiref(t, line.Text == "two", "left movement = %q", line.Text)
	rightAtContext, _ := v.switchPane(nextLeft, right)
	nextRight, _ := v.move(rightAtContext, forward)
	line, _ = v.line(nextRight)
	testutil.Requiref(t, line.Text == "three", "right movement = %q", line.Text)
}

func TestSplitVerticalMovementAndHalfPageUseVisualRows(t *testing.T) {
	lines := []patch.Line{
		{Kind: patch.Deletion, Text: "d1", OldNumber: 1}, {Kind: patch.Addition, Text: "a1", NewNumber: 1}, {Kind: patch.Context, Text: "c1", OldNumber: 2, NewNumber: 2},
		{Kind: patch.Deletion, Text: "d2", OldNumber: 3}, {Kind: patch.Addition, Text: "a2", NewNumber: 3}, {Kind: patch.Context, Text: "c2", OldNumber: 4, NewNumber: 4},
		{Kind: patch.Deletion, Text: "d3", OldNumber: 5}, {Kind: patch.Addition, Text: "a3", NewNumber: 5}, {Kind: patch.Context, Text: "c3", OldNumber: 6, NewNumber: 6},
	}
	v := newSideBySideView(patch.Patch{Files: []patch.File{{DisplayPath: "file", Hunks: []patch.Hunk{{Header: "@@", Lines: lines}}}}}, true)
	cursor := mustFirst(t, v)
	viewport := v.newViewport(120, 4)
	viewport = v.keepVisible(viewport, cursor)
	originalTop := viewport.top
	viewport, moved := v.scrollHalfPage(viewport, cursor, forward)
	testutil.Requiref(t, viewport.top > originalTop && moved.row > cursor.row, "viewport=%#v cursor=%#v", viewport, moved)
	viewport, moved = v.scrollHalfPage(viewport, moved, backward)
	testutil.Requiref(t, viewport.top == originalTop && moved.row == cursor.row, "round trip viewport=%#v cursor=%#v", viewport, moved)
}

func TestFileHeaderSticksWithoutCoveringDiffRows(t *testing.T) {
	p := patch.Patch{Files: []patch.File{
		{DisplayPath: "first.go", Hunks: []patch.Hunk{{Header: "@@", Lines: []patch.Line{
			{Kind: patch.Context, Text: "first one", OldNumber: 1, NewNumber: 1},
			{Kind: patch.Context, Text: "first two", OldNumber: 2, NewNumber: 2},
			{Kind: patch.Context, Text: "first three", OldNumber: 3, NewNumber: 3},
		}}}},
		{DisplayPath: "second.go", Hunks: []patch.Hunk{{Header: "@@", Lines: []patch.Line{
			{Kind: patch.Context, Text: "second one", OldNumber: 1, NewNumber: 1},
		}}}},
	}}
	v := newUnifiedView(p, true)
	viewport := v.newViewport(60, 3)
	viewport.top = 3

	rendered := strings.Split(ansi.Strip(v.render(viewport, diffCursor{}, nil)), "\n")
	testutil.Requiref(t, len(rendered) == viewport.Height && strings.Contains(rendered[0], "first.go"), "sticky render = %#v", rendered)
	testutil.Requiref(t, strings.Contains(rendered[1], "first two") && strings.Contains(rendered[2], "first three"), "sticky header covered content rows: %#v", rendered)

	secondFileRow := 5
	viewport.top = secondFileRow
	rendered = strings.Split(ansi.Strip(v.render(viewport, diffCursor{}, nil)), "\n")
	testutil.Requiref(t, strings.Count(strings.Join(rendered, "\n"), "second.go") == 1, "file header was duplicated at its natural position: %#v", rendered)

	viewport.top = secondFileRow + 1
	rendered = strings.Split(ansi.Strip(v.render(viewport, diffCursor{}, nil)), "\n")
	testutil.Requiref(t, strings.Contains(rendered[0], "second.go"), "sticky header did not change with the file: %#v", rendered)
}

func TestKeepVisibleAccountsForStickyFileHeader(t *testing.T) {
	v := newUnifiedView(longPatch(), true)
	cursor, _ := v.last()
	viewport := v.keepVisible(v.newViewport(50, 4), cursor)
	rendered := ansi.Strip(v.render(viewport, cursor, nil))
	line, _ := v.line(cursor)
	testutil.Requiref(t, cursor.row < viewport.top+v.contentHeight(viewport) && strings.Contains(rendered, strconv.Itoa(int(line.NewNumber))), "last cursor row is hidden by sticky header: viewport=%#v render=%q", viewport, rendered)
}

func TestSplitTabsDoNotShiftLineNumbersOrDivider(t *testing.T) {
	p := patch.Patch{Files: []patch.File{{DisplayPath: "file", Hunks: []patch.Hunk{{Header: "@@", Lines: []patch.Line{{Kind: patch.Context, Text: "\t\tlong line", OldNumber: 1, NewNumber: 1}}}}}}}
	v := newSideBySideView(p, true)
	cursor := mustFirst(t, v)
	rendered := ansi.Strip(renderOne(v, cursor, 120, nil))
	testutil.Requiref(t, !strings.ContainsRune(rendered, '\t') && strings.Index(rendered, "│") == 59 && lipgloss.Width(rendered) == 120, "rendered = %q width=%d", rendered, lipgloss.Width(rendered))
}

func TestHorizontalScrollKeepsUnifiedGutterFixed(t *testing.T) {
	p := longLinePatch()
	v := newUnifiedView(p, true)
	cursor := mustFirst(t, v)
	viewport := v.newViewport(37, 1)
	viewport = v.keepVisible(viewport, cursor)
	before := ansi.Strip(v.render(viewport, cursor, nil))
	viewport = v.scrollHorizontal(viewport, 4)
	after := ansi.Strip(v.render(viewport, cursor, nil))
	testutil.Requiref(t, before[:14] == after[:14] && strings.Contains(after[14:], "efghij") && viewport.LeftColumn == 4, "before=%q after=%q viewport=%#v", before, after, viewport)
}

func TestHorizontalScrollKeepsSplitGuttersAndDividerFixed(t *testing.T) {
	v := newSideBySideView(longLinePatch(), true)
	cursor := mustFirst(t, v)
	viewport := v.newViewport(120, 1)
	viewport = v.keepVisible(viewport, cursor)
	viewport = v.scrollHorizontal(viewport, 8)
	rendered := ansi.Strip(v.render(viewport, cursor, nil))
	testutil.Requiref(t, strings.Index(rendered, "│") == 59 && rendered[:6] == "    1 " && rendered[63:69] == "    1 ", "gutters moved: %q", rendered)
	testutil.Requiref(t, strings.Contains(rendered[6:59], "ghijkl") && strings.Contains(rendered[69:], "ghijkl"), "offset differs: %q", rendered)
}

func TestHorizontalScrollStartAndEndClamp(t *testing.T) {
	v := newUnifiedView(longLinePatch(), true)
	viewport := v.newViewport(37, 1)
	viewport = v.scrollHorizontal(viewport, int(^uint(0)>>1))
	testutil.Require(t, viewport.LeftColumn != 0, "end did not scroll")
	viewport = v.scrollHorizontal(viewport, -int(^uint(0)>>1))
	testutil.Requiref(t, viewport.LeftColumn == 0, "start = %d", viewport.LeftColumn)
}

func TestDiffMarkersUseTerminalColorsAndCursorFillsWidth(t *testing.T) {
	v := newUnifiedView(testPatch(), true)
	first := mustFirst(t, v)
	added, _ := v.search("added one", first, forward)
	removed, _ := v.search("removed one", first, forward)
	addedRender := renderTarget(v, added, first, 80, nil)
	removedRender := renderTarget(v, removed, first, 80, nil)
	testutil.Requiref(t, strings.Contains(addedRender, "\x1b[32m+\x1b[m") && strings.Contains(removedRender, "\x1b[31m-\x1b[m"), "added=%q removed=%q", addedRender, removedRender)
	assertStyledThroughColumn(t, renderOne(v, added, 80, nil), 80, sgrExpectation{reverse: true})
}

func TestSelectionBackgroundKeepsDefaultWeight(t *testing.T) {
	v := newUnifiedView(testPatch(), false)
	first := mustFirst(t, v)
	removed, _ := v.search("removed one", first, forward)
	selection := v.beginSelection(removed)
	rendered := renderTarget(v, removed, first, 72, &selection)
	testutil.Requiref(t, !strings.Contains(rendered, "\x1b[1m"), "selection is bold: %q", rendered)
	assertStyledThroughColumn(t, rendered, 72, sgrExpectation{background: "219;234;254"})
}

func TestSyntaxHighlightingSurvivesDiffStyling(t *testing.T) {
	p := patch.Patch{Files: []patch.File{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", OldSource: "package main\nold()\n", NewSource: "package main\nnew()\n", Hunks: []patch.Hunk{{Header: "@@", Lines: []patch.Line{{Kind: patch.Deletion, Text: "old()", OldNumber: 2}, {Kind: patch.Addition, Text: "new()", NewNumber: 2}}}}}}}
	v := newUnifiedView(p, true)
	first := mustFirst(t, v)
	added, _ := v.search("new()", first, forward)
	for _, cursor := range []diffCursor{first, added} {
		rendered := renderTarget(v, cursor, diffCursor{}, 80, nil)
		testutil.Requiref(t, strings.Contains(rendered, "[38;2;"), "syntax highlighting missing: %q", rendered)
	}
}

func TestRenderedCodeRowsHaveExactTerminalWidth(t *testing.T) {
	for _, test := range []struct {
		constructor func(patch.Patch, bool) *diffView
		width       int
	}{{newUnifiedView, 37}, {newSideBySideView, 120}} {
		v := test.constructor(testPatch(), true)
		assertCodeRowWidths(t, v, test.width)
	}
}

func TestRenderStyledRowStripsSyntaxBackgroundColors(t *testing.T) {
	value := strings.Join([]string{"\x1b[48;2;255;0;0;38;2;1;2;3mtruecolor", "\x1b[48;5;123;1mindexed", "\x1b[45mstandard", "\x1b[105mbright"}, " ")
	rendered := renderStyledRow(lineStyle(patch.Addition, true), value, 80, false)
	for _, forbidden := range []string{"48;2;255;0;0", "48;5;123", "[45m", "[105m"} {
		testutil.Requiref(t, !strings.Contains(rendered, forbidden), "retains %q: %q", forbidden, rendered)
	}
}

func renderOne(v *diffView, cursor diffCursor, width int, selection *diffSelection) string {
	return renderTarget(v, cursor, cursor, width, selection)
}

func renderTarget(v *diffView, target, active diffCursor, width int, selection *diffSelection) string {
	viewport := v.newViewport(width, 1)
	viewport.top = target.row
	return v.render(viewport, active, selection)
}

func longLinePatch() patch.Patch {
	return patch.Patch{Files: []patch.File{{DisplayPath: "long", Hunks: []patch.Hunk{{Header: "@@", Lines: []patch.Line{{Kind: patch.Context, Text: "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ", OldNumber: 1, NewNumber: 1}}}}}}}
}

type sgrExpectation struct {
	background string
	reverse    bool
}
type sgrState struct {
	background string
	reverse    bool
}

var sgrPattern = regexp.MustCompile(`\x1b\[([0-9;]*)m`)

func assertStyledThroughColumn(t *testing.T, rendered string, width int, expected sgrExpectation) {
	t.Helper()
	state := sgrState{}
	column := 0
	for len(rendered) > 0 {
		text, parameters, rest := nextSGRSegment(rendered)
		column = assertStyledText(t, text, column, state, expected)
		state.apply(parameters)
		rendered = rest
	}
	testutil.Requiref(t, column == width, "styled columns = %d, want %d", column, width)
}

func nextSGRSegment(rendered string) (string, string, string) {
	location := sgrPattern.FindStringSubmatchIndex(rendered)
	if location == nil {
		return rendered, "", ""
	}
	return rendered[:location[0]], rendered[location[2]:location[3]], rendered[location[1]:]
}

func assertStyledText(t *testing.T, text string, column int, state sgrState, expected sgrExpectation) int {
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
		index = s.applyCode(value, values, index)
	}
}
func assertSGRState(t *testing.T, column int, state sgrState, expected sgrExpectation) {
	t.Helper()
	testutil.Requiref(t, state.background == expected.background && state.reverse == expected.reverse, "column %d state=%#v want=%#v", column, state, expected)
}

func codeEntries(v *diffView) []entry {
	var code []entry
	for _, current := range v.rows {
		if current.kind == lineRow {
			code = append(code, current)
		}
	}
	return code
}

func assertCodeRowWidths(t *testing.T, v *diffView, expected int) {
	t.Helper()
	for cursor, ok := v.first(); ok; cursor, ok = v.move(cursor, forward) {
		width := lipgloss.Width(renderOne(v, cursor, expected, nil))
		testutil.Requiref(t, width == expected, "width = %d", width)
	}
}

func (s *sgrState) applyCode(value int, values []string, index int) int {
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
		return s.applyBackground(values, index)
	}
	return index
}

func (s *sgrState) applyBackground(values []string, index int) int {
	if index+4 < len(values) && values[index+1] == "2" {
		s.background = strings.Join(values[index+2:index+5], ";")
		return index + 4
	}
	return index
}
