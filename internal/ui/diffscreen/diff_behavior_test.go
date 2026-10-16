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
	v := newSideBySideView(makeUnequalChangeBlockPatch(), true)
	var code []entry
	for _, current := range v.rows {
		if current.kind == lineRow {
			code = append(code, current)
		}
	}
	testutil.RequireValuesEqual(t, len(code), 5, "visual code entries = %d",
		len(code),
	)
	testutil.RequireValuesEqual(
		t,
		code[0].leftLine,
		0,
		"pairing = %#v",
		code[:3],
	)
	testutil.RequireValuesEqual(
		t,
		code[0].rightLine,
		2,
		"pairing = %#v",
		code[:3],
	)
	testutil.RequireValuesEqual(
		t,
		code[1].leftLine,
		1,
		"pairing = %#v",
		code[:3],
	)
	testutil.RequireValuesEqual(
		t,
		code[1].rightLine,
		3,
		"pairing = %#v",
		code[:3],
	)
	testutil.RequireValuesEqual(
		t,
		code[2].leftLine,
		-1,
		"pairing = %#v",
		code[:3],
	)
	testutil.RequireValuesEqual(
		t,
		code[2].rightLine,
		4,
		"pairing = %#v",
		code[:3],
	)
	testutil.RequireValuesEqual(t, code[4].hunk != code[3].hunk, true,
		"hunks were paired: %#v", code[4],
	)
	testutil.RequireValuesEqual(
		t,
		code[4].leftLine,
		-1,
		"hunks were paired: %#v",
		code[4],
	)
}

func makeUnequalChangeBlockPatch() patch.Patch {
	return patch.Patch{
		Files: []patch.File{{DisplayPath: "file", Hunks: []patch.Hunk{
			{
				Header: "one",
				Lines: []patch.Line{
					{Kind: patch.Deletion, Text: "d1", OldNumber: 1},
					{Kind: patch.Deletion, Text: "d2", OldNumber: 2},
					{Kind: patch.Addition, Text: "a1", NewNumber: 1},
					{Kind: patch.Addition, Text: "a2", NewNumber: 2},
					{Kind: patch.Addition, Text: "a3", NewNumber: 3},
					{
						Kind:      patch.Context,
						Text:      "c",
						OldNumber: 3,
						NewNumber: 4,
					},
				},
			},
			{
				Header: "two",
				Lines: []patch.Line{
					{Kind: patch.Addition, Text: "separate", NewNumber: 5},
				},
			},
		}}},
	}
}

func TestSplitSelectionOnlyIncludesActivePane(t *testing.T) {
	v := newSideBySideView(testPatch(), true)
	first := mustFirst(t, v)
	removed, _ := v.findTextInChanges("removed one", first, forward)
	added, _ := v.findTextInChanges("added one", first, forward)
	left := beginSelection(removed)
	left, ok := v.extendSelection(left, removed)
	testutil.RequireValuesEqual(
		t,
		ok,
		true,
		"left lines = %#v",
		v.selectedLines(left),
	)
	testutil.RequireValuesEqual(
		t,
		len(v.selectedLines(left)),
		1,
		"left lines = %#v",
		v.selectedLines(left),
	)
	testutil.RequireValuesEqual(
		t,
		v.selectedLines(left)[0].Kind,
		patch.Deletion,
		"left lines = %#v",
		v.selectedLines(left),
	)

	right := beginSelection(added)
	testutil.RequireValuesEqual(
		t,
		len(v.selectedLines(right)),
		1,
		"right lines = %#v",
		v.selectedLines(right),
	)
	testutil.RequireValuesEqual(
		t,
		v.selectedLines(right)[0].Kind,
		patch.Addition,
		"right lines = %#v",
		v.selectedLines(right),
	)
}

func TestSplitPaneSwitchingFindsRowsAboveAndBelowEmptyTargets(t *testing.T) {
	p := patch.Patch{
		Files: []patch.File{
			{
				DisplayPath: "file",
				Hunks: []patch.Hunk{{Header: "@@", Lines: []patch.Line{
					{Kind: patch.Addition, Text: "right", NewNumber: 1},
					{
						Kind:      patch.Context,
						Text:      "both",
						OldNumber: 1,
						NewNumber: 2,
					},
					{Kind: patch.Deletion, Text: "left", OldNumber: 2},
				}}},
			},
		},
	}
	v := newSideBySideView(p, true)
	first := mustFirst(t, v)
	left, ok := v.switchToPane(first, left)
	testutil.RequireValuesEqual(
		t,
		ok,
		true,
		"%s",
		"did not find later left pane",
	)

	line, _ := v.patchLineForPosition(left)
	testutil.RequireValuesEqual(t, line.Text, "both", "later left line = %q",
		line.Text,
	)

	last, _ := v.lastSelectableLine()
	right, ok := v.switchToPane(last, right)
	testutil.RequireValuesEqual(
		t,
		ok,
		true,
		"%s",
		"did not find earlier right pane",
	)

	line, _ = v.patchLineForPosition(right)
	testutil.RequireValuesEqual(t, line.Text, "both", "earlier right line = %q",
		line.Text,
	)
}

func TestSplitPaneSwitchDoesNothingWhenTargetPaneIsEmpty(t *testing.T) {
	p := patch.Patch{
		Files: []patch.File{
			{
				DisplayPath: "file",
				Hunks: []patch.Hunk{
					{
						Header: "@@",
						Lines: []patch.Line{
							{Kind: patch.Addition, Text: "one", NewNumber: 1},
							{Kind: patch.Addition, Text: "two", NewNumber: 2},
						},
					},
				},
			},
		},
	}
	v := newSideBySideView(p, true)
	cursor := mustFirst(t, v)
	if _, ok := v.switchToPane(cursor, left); ok {
		t.Fatal("switched to empty pane")
	}
}

func TestSplitVerticalMovementSkipsEmptyActivePane(t *testing.T) {
	p := patch.Patch{
		Files: []patch.File{
			{
				DisplayPath: "file",
				Hunks: []patch.Hunk{
					{
						Header: "@@",
						Lines: []patch.Line{
							{
								Kind:      patch.Context,
								Text:      "one",
								OldNumber: 1,
								NewNumber: 1,
							},
							{Kind: patch.Addition, Text: "right", NewNumber: 2},
							{
								Kind:      patch.Context,
								Text:      "two",
								OldNumber: 2,
								NewNumber: 3,
							},
							{Kind: patch.Deletion, Text: "left", OldNumber: 3},
							{
								Kind:      patch.Context,
								Text:      "three",
								OldNumber: 4,
								NewNumber: 4,
							},
						},
					},
				},
			},
		},
	}
	v := newSideBySideView(p, true)
	first := mustFirst(t, v)
	left, _ := v.switchToPane(first, left)
	nextLeft, _ := v.moveInDirection(left, forward)
	line, _ := v.patchLineForPosition(nextLeft)
	testutil.RequireValuesEqual(
		t,
		line.Text,
		"two",
		"left movement = %q",
		line.Text,
	)

	rightAtContext, _ := v.switchToPane(nextLeft, right)
	nextRight, _ := v.moveInDirection(rightAtContext, forward)
	line, _ = v.patchLineForPosition(nextRight)
	testutil.RequireValuesEqual(t, line.Text, "three", "right movement = %q",
		line.Text,
	)
}

func TestSplitVerticalMovementAndHalfPageUseVisualRows(t *testing.T) {
	lines := []patch.Line{
		{
			Kind:      patch.Deletion,
			Text:      "d1",
			OldNumber: 1,
		}, {
			Kind:      patch.Addition,
			Text:      "a1",
			NewNumber: 1,
		}, {Kind: patch.Context, Text: "c1", OldNumber: 2, NewNumber: 2},
		{
			Kind:      patch.Deletion,
			Text:      "d2",
			OldNumber: 3,
		}, {
			Kind:      patch.Addition,
			Text:      "a2",
			NewNumber: 3,
		}, {Kind: patch.Context, Text: "c2", OldNumber: 4, NewNumber: 4},
		{
			Kind:      patch.Deletion,
			Text:      "d3",
			OldNumber: 5,
		}, {
			Kind:      patch.Addition,
			Text:      "a3",
			NewNumber: 5,
		}, {Kind: patch.Context, Text: "c3", OldNumber: 6, NewNumber: 6},
	}
	v := newSideBySideView(
		patch.Patch{
			Files: []patch.File{
				{
					DisplayPath: "file",
					Hunks:       []patch.Hunk{{Header: "@@", Lines: lines}},
				},
			},
		},
		true,
	)
	cursor := mustFirst(t, v)
	viewport := v.newVisibleArea(120, 4)
	viewport = v.keepFocusedLineVisible(viewport, cursor)
	originalTop := viewport.top
	viewport, moved := v.moveFocusedLineHalfPage(viewport, cursor, forward)
	testutil.RequireValuesEqual(t, viewport.top > originalTop, true,
		"viewport=%#v cursor=%#v", viewport, moved,
	)
	testutil.RequireValuesEqual(t, moved.row > cursor.row, true,
		"viewport=%#v cursor=%#v", viewport, moved,
	)

	viewport, moved = v.moveFocusedLineHalfPage(viewport, moved, backward)
	testutil.RequireValuesEqual(t, viewport.top, originalTop,
		"round trip viewport=%#v cursor=%#v", viewport, moved,
	)
	testutil.RequireValuesEqual(t, moved.row, cursor.row,
		"round trip viewport=%#v cursor=%#v", viewport, moved,
	)
}

func TestFileHeaderSticksWithoutCoveringDiffRows(t *testing.T) {
	p := stickyHeaderPatch()
	v := newUnifiedView(p, true)
	viewport := v.newVisibleArea(60, 3)
	viewport.top = 3

	rendered := strings.Split(
		ansi.Strip(v.formatVisibleChanges(viewport, diffCursor{}, nil)),
		"\n",
	)
	testutil.RequireValuesEqual(t, len(rendered), viewport.Height,
		"sticky render = %#v", rendered,
	)
	testutil.RequireValuesEqual(
		t,
		strings.Contains(rendered[0], "first.go"),
		true,
		"sticky render = %#v",
		rendered,
	)
	testutil.RequireValuesEqual(
		t,
		strings.Contains(rendered[1], "first two"),
		true,
		"sticky header covered content rows: %#v",
		rendered,
	)
	testutil.RequireValuesEqual(
		t,
		strings.Contains(rendered[2], "first three"),
		true,
		"sticky header covered content rows: %#v",
		rendered,
	)

	secondFileRow := 5
	viewport.top = secondFileRow
	rendered = strings.Split(
		ansi.Strip(v.formatVisibleChanges(viewport, diffCursor{}, nil)),
		"\n",
	)
	testutil.RequireValuesEqual(t, strings.Count(
		strings.Join(rendered, "\n"),
		"second.go",
	), 1,
		"file header was duplicated at its natural position: %#v", rendered)

	viewport.top = secondFileRow + 1
	rendered = strings.Split(
		ansi.Strip(v.formatVisibleChanges(viewport, diffCursor{}, nil)),
		"\n",
	)
	testutil.RequireValuesEqual(
		t,
		strings.Contains(rendered[0], "second.go"),
		true,
		"sticky header did not change with the file: %#v",
		rendered,
	)
}

func stickyHeaderPatch() patch.Patch {
	p := patch.Patch{Files: []patch.File{
		{
			DisplayPath: "first.go",
			Hunks: []patch.Hunk{{Header: "@@", Lines: []patch.Line{
				{
					Kind:      patch.Context,
					Text:      "first one",
					OldNumber: 1,
					NewNumber: 1,
				},
				{
					Kind:      patch.Context,
					Text:      "first two",
					OldNumber: 2,
					NewNumber: 2,
				},
				{
					Kind:      patch.Context,
					Text:      "first three",
					OldNumber: 3,
					NewNumber: 3,
				},
			}}},
		},
		{
			DisplayPath: "second.go",
			Hunks: []patch.Hunk{{Header: "@@", Lines: []patch.Line{
				{
					Kind:      patch.Context,
					Text:      "second one",
					OldNumber: 1,
					NewNumber: 1,
				},
			}}},
		},
	}}
	return p
}

func TestKeepVisibleAccountsForStickyFileHeader(t *testing.T) {
	v := newUnifiedView(longPatch(), true)
	cursor, _ := v.lastSelectableLine()
	viewport := v.keepFocusedLineVisible(v.newVisibleArea(50, 4), cursor)
	rendered := ansi.Strip(v.formatVisibleChanges(viewport, cursor, nil))
	line, _ := v.patchLineForPosition(cursor)
	testutil.RequireValuesEqual(t,
		cursor.row < viewport.top+v.visibleRowsBelowHeader(viewport), true,
		"last cursor row is hidden by sticky header: viewport=%#v render=%q",
		viewport, rendered,
	)
	testutil.RequireValuesEqual(t,
		strings.Contains(rendered, strconv.Itoa(int(line.NewNumber))), true,
		"last cursor row is hidden by sticky header: viewport=%#v render=%q",
		viewport, rendered,
	)
}

func TestSplitTabsDoNotShiftLineNumbersOrDivider(t *testing.T) {
	p := patch.Patch{
		Files: []patch.File{
			{
				DisplayPath: "file",
				Hunks: []patch.Hunk{
					{
						Header: "@@",
						Lines: []patch.Line{
							{
								Kind:      patch.Context,
								Text:      "\t\tlong line",
								OldNumber: 1,
								NewNumber: 1,
							},
						},
					},
				},
			},
		},
	}
	v := newSideBySideView(p, true)
	cursor := mustFirst(t, v)
	rendered := ansi.Strip(renderOne(v, cursor, 120, nil))
	testutil.RequireValuesEqual(t, !strings.ContainsRune(rendered, '\t'), true,
		"rendered = %q width=%d", rendered, lipgloss.Width(rendered),
	)
	testutil.RequireValuesEqual(t, strings.Index(rendered, "│"), 59,
		"rendered = %q width=%d", rendered, lipgloss.Width(rendered),
	)
	testutil.RequireValuesEqual(t, lipgloss.Width(rendered), 120,
		"rendered = %q width=%d", rendered, lipgloss.Width(rendered),
	)
}

func TestHorizontalScrollKeepsUnifiedGutterFixed(t *testing.T) {
	p := longLinePatch()
	v := newUnifiedView(p, true)
	cursor := mustFirst(t, v)
	viewport := v.newVisibleArea(37, 1)
	viewport = v.keepFocusedLineVisible(viewport, cursor)
	before := ansi.Strip(v.formatVisibleChanges(viewport, cursor, nil))
	viewport = v.scrollVisibleAreaSideways(viewport, 4)
	after := ansi.Strip(v.formatVisibleChanges(viewport, cursor, nil))
	testutil.RequireValuesEqual(t, before[:14], after[:14],
		"before=%q after=%q viewport=%#v", before, after, viewport,
	)
	testutil.RequireValuesEqual(t, strings.Contains(after[14:], "efghij"), true,
		"before=%q after=%q viewport=%#v", before, after, viewport,
	)
	testutil.RequireValuesEqual(t, viewport.LeftColumn, 4,
		"before=%q after=%q viewport=%#v", before, after, viewport,
	)
}

func TestHorizontalScrollKeepsSplitGuttersAndDividerFixed(t *testing.T) {
	v := newSideBySideView(longLinePatch(), true)
	cursor := mustFirst(t, v)
	viewport := v.newVisibleArea(120, 1)
	viewport = v.keepFocusedLineVisible(viewport, cursor)
	viewport = v.scrollVisibleAreaSideways(viewport, 8)
	rendered := ansi.Strip(v.formatVisibleChanges(viewport, cursor, nil))
	testutil.RequireValuesEqual(t, strings.Index(rendered, "│"), 59,
		"gutters moved: %q", rendered,
	)
	testutil.RequireValuesEqual(t, rendered[:6], "    1 ", "gutters moved: %q",
		rendered,
	)
	testutil.RequireValuesEqual(
		t,
		rendered[63:69],
		"    1 ",
		"gutters moved: %q",
		rendered,
	)
	testutil.RequireValuesEqual(
		t,
		strings.Contains(rendered[6:59], "ghijkl"),
		true,
		"offset differs: %q",
		rendered,
	)
	testutil.RequireValuesEqual(
		t,
		strings.Contains(rendered[69:], "ghijkl"),
		true,
		"offset differs: %q",
		rendered,
	)
}

func TestHorizontalScrollStartAndEndClamp(t *testing.T) {
	v := newUnifiedView(longLinePatch(), true)
	viewport := v.newVisibleArea(37, 1)
	viewport = v.scrollVisibleAreaSideways(viewport, int(^uint(0)>>1))
	testutil.RequireValuesEqual(t, viewport.LeftColumn != 0, true, "%s",
		"end did not scroll",
	)

	viewport = v.scrollVisibleAreaSideways(viewport, -int(^uint(0)>>1))
	testutil.RequireValuesEqual(t, viewport.LeftColumn, 0, "start = %d",
		viewport.LeftColumn,
	)
}

func TestDiffMarkersUseTerminalColorsAndCursorFillsWidth(t *testing.T) {
	v := newUnifiedView(testPatch(), true)
	first := mustFirst(t, v)
	added, _ := v.findTextInChanges("added one", first, forward)
	removed, _ := v.findTextInChanges("removed one", first, forward)
	addedRender := renderTarget(v, added, first, 80, nil)
	removedRender := renderTarget(v, removed, first, 80, nil)
	testutil.RequireValuesEqual(
		t,
		strings.Contains(addedRender, "\x1b[32m+\x1b[m"),
		true,
		"added=%q removed=%q",
		addedRender,
		removedRender,
	)
	testutil.RequireValuesEqual(
		t,
		strings.Contains(removedRender, "\x1b[31m-\x1b[m"),
		true,
		"added=%q removed=%q",
		addedRender,
		removedRender,
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
	removed, _ := v.findTextInChanges("removed one", first, forward)
	selection := beginSelection(removed)
	rendered := renderTarget(v, removed, first, 72, &selection)
	testutil.RequireValuesEqual(t, !strings.Contains(rendered, "\x1b[1m"), true,
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
	added, _ := v.findTextInChanges("new()", first, forward)
	for _, cursor := range []diffCursor{first, added} {
		rendered := renderTarget(v, cursor, diffCursor{}, 80, nil)
		testutil.RequireValuesEqual(
			t,
			strings.Contains(rendered, "[38;2;"),
			true,
			"syntax highlighting missing: %q",
			rendered,
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
			testutil.RequireValuesEqual(
				t,
				width,
				test.width,
				"width=%d want=%d",
				width,
				test.width,
			)
			next, ok := v.moveInDirection(cursor, forward)
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
	rendered := formatStyledLine(
		styleForChange(patch.Addition, true),
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
		testutil.RequireValuesEqual(
			t,
			!strings.Contains(rendered, forbidden),
			true,
			"retains %q: %q",
			forbidden,
			rendered,
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
	viewport := v.newVisibleArea(width, 1)
	viewport.top = target.row
	return v.formatVisibleChanges(viewport, active, selection)
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

type sgrExpectation struct {
	background string
	reverse    bool
}
type sgrState struct {
	background string
	reverse    bool
}

var sgrPattern = regexp.MustCompile(`\x1b\[([0-9;]*)m`)

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
	testutil.RequireValuesEqual(
		t,
		column,
		width,
		"styled columns = %d, want %d",
		column,
		width,
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
	testutil.RequireValuesEqual(t, state.background, expected.background,
		"column %d state=%#v want=%#v", column, state, expected,
	)
	testutil.RequireValuesEqual(t, state.reverse, expected.reverse,
		"column %d state=%#v want=%#v", column, state, expected,
	)
}
