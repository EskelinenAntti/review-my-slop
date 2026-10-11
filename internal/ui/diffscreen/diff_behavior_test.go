package diffscreen

import (
	"regexp"
	"strconv"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

func TestSplitPairsUnequalChangeBlocksAndKeepsHunksSeparate(t *testing.T) {
	p := unequalChangePatch()
	v := newSideBySideView(p, true)
	var code []entry
	for _, current := range v.rows {
		if current.kind == lineRow {
			code = append(code, current)
		}
	}
	requireEqual(t, len(code), five, "visual code entries = %d", len(code))
	requireEqual(
		t,
		code[zero].leftLine == zero && code[zero].rightLine == two &&
			code[one].leftLine == one &&
			code[one].rightLine == three &&
			code[two].leftLine == -one &&
			code[two].rightLine == four,
		true,
		"pairing = %#v",
		code[:three],
	)
	requireEqual(
		t,
		code[four].hunk != code[three].hunk && code[four].leftLine == -one,
		true,
		"hunks were paired: %#v",
		code[four],
	)
}

func TestSplitSelectionOnlyIncludesActivePane(t *testing.T) {
	v := newSideBySideView(testPatch(), true)
	first := mustFirst(t, v)
	removed, _ := v.search(fixtureRemovedOne, first, forward)
	added, _ := v.search("added one", first, forward)
	left := v.beginSelection(removed)
	left, ok := v.extendSelection(left, removed)
	requireEqual(
		t,
		ok && len(v.lines(left)) == one &&
			v.lines(left)[zero].Kind == patch.Deletion,
		true,
		"left lines = %#v",
		v.lines(left),
	)
	right := v.beginSelection(added)
	requireEqual(
		t,
		len(v.lines(right)) == one &&
			v.lines(right)[zero].Kind == patch.Addition,
		true,
		"right lines = %#v",
		v.lines(right),
	)
}

func TestSplitPaneSwitchingFindsRowsAboveAndBelowEmptyTargets(t *testing.T) {
	p := patch.Patch{
		Files: []patch.File{
			{
				DisplayPath: fixtureFile,

				Hunks: []patch.Hunk{
					{
						Header: hunkMarker,

						Lines: []patch.Line{
							{
								Kind:      patch.Addition,
								Text:      "right",
								NewNumber: one,
							},

							{
								Kind:      patch.Context,
								Text:      fixtureBoth,
								OldNumber: one,
								NewNumber: two,
							},

							{
								Kind:      patch.Deletion,
								Text:      "left",
								OldNumber: two,
							},
						},
					},
				},
			},
		},
	}
	v := newSideBySideView(p, true)
	first := mustFirst(t, v)
	left, ok := v.switchPane(first, left)
	requireEqual(t, ok, true, fixtureV, "did not find later left pane")
	line, _ := v.line(left)
	requireEqual(t, line.Text, fixtureBoth, "later left line = %q", line.Text)
	last, _ := v.last()
	right, ok := v.switchPane(last, right)
	requireEqual(t, ok, true, fixtureV, "did not find earlier right pane")
	line, _ = v.line(right)
	requireEqual(
		t,
		line.Text, fixtureBoth,
		"earlier right line = %q",
		line.Text,
	)
}

func TestSplitPaneSwitchDoesNothingWhenTargetPaneIsEmpty(t *testing.T) {
	p := patch.Patch{
		Files: []patch.File{
			{
				DisplayPath: fixtureFile,
				Hunks: []patch.Hunk{
					{
						Header: hunkMarker,
						Lines: []patch.Line{
							{
								Kind:      patch.Addition,
								Text:      fixtureOne,
								NewNumber: one,
							},
							{
								Kind:      patch.Addition,
								Text:      fixtureTwo,
								NewNumber: two,
							},
						},
					},
				},
			},
		},
	}
	v := newSideBySideView(p, true)
	cursor := mustFirst(t, v)
	{
		_, ok := v.switchPane(cursor, left)
		requireEqual(t, !ok, true, fixtureV, "switched to empty pane")
	}
}

func TestSplitVerticalMovementSkipsEmptyActivePane(t *testing.T) {
	p := patch.Patch{
		Files: []patch.File{
			{
				DisplayPath: fixtureFile,
				Hunks: []patch.Hunk{
					{
						Header: hunkMarker,
						Lines: []patch.Line{
							{
								Kind:      patch.Context,
								Text:      fixtureOne,
								OldNumber: one,
								NewNumber: one,
							},
							{
								Kind:      patch.Addition,
								Text:      "right",
								NewNumber: two,
							},
							{
								Kind:      patch.Context,
								Text:      fixtureTwo,
								OldNumber: two,
								NewNumber: three,
							},
							{
								Kind:      patch.Deletion,
								Text:      "left",
								OldNumber: three,
							},
							{
								Kind:      patch.Context,
								Text:      "three",
								OldNumber: four,
								NewNumber: four,
							},
						},
					},
				},
			},
		},
	}
	v := newSideBySideView(p, true)
	first := mustFirst(t, v)
	left, _ := v.switchPane(first, left)
	nextLeft, _ := v.move(left, forward)
	line, _ := v.line(nextLeft)
	requireEqual(t, line.Text, fixtureTwo, "left movement = %q", line.Text)
	rightAtContext, _ := v.switchPane(nextLeft, right)
	nextRight, _ := v.move(rightAtContext, forward)
	line, _ = v.line(nextRight)
	requireEqual(t, line.Text, "three", "right movement = %q", line.Text)
}

func TestSplitVerticalMovementAndHalfPageUseVisualRows(t *testing.T) {
	lines := []patch.Line{
		{Kind: patch.Deletion, Text: "d1", OldNumber: one},
		{Kind: patch.Addition, Text: "a1", NewNumber: one},
		{Kind: patch.Context, Text: "c1", OldNumber: two, NewNumber: two},

		{Kind: patch.Deletion, Text: "d2", OldNumber: three},
		{Kind: patch.Addition, Text: "a2", NewNumber: three},
		{Kind: patch.Context, Text: "c2", OldNumber: four, NewNumber: four},

		{Kind: patch.Deletion, Text: "d3", OldNumber: five},
		{Kind: patch.Addition, Text: "a3", NewNumber: five},
		{Kind: patch.Context, Text: "c3", OldNumber: six, NewNumber: six},
	}
	v := newSideBySideView(

		patch.Patch{
			Files: []patch.File{
				{
					DisplayPath: fixtureFile,

					Hunks: []patch.Hunk{
						{
							Header: hunkMarker,
							Lines:  lines,
						},
					},
				},
			},
		},

		true,
	)
	cursor := mustFirst(t, v)
	viewport := v.newViewport(wideTestWidth, four)
	viewport = v.keepVisible(viewport, cursor)
	originalTop := viewport.top
	viewport, moved := v.scrollHalfPage(viewport, cursor, forward)
	requireEqual(
		t,
		viewport.top > originalTop && moved.row > cursor.row, true,
		"viewport=%#v cursor=%#v",
		viewport,
		moved,
	)
	viewport, moved = v.scrollHalfPage(viewport, moved, backward)
	requireEqual(
		t,
		viewport.top == originalTop && moved.row == cursor.row, true,
		"round trip viewport=%#v cursor=%#v",
		viewport,
		moved,
	)
}

func TestFileHeaderSticksWithoutCoveringDiffRows(t *testing.T) {
	p := stickyHeaderPatch()
	v := newUnifiedView(p, true)
	viewport := v.newViewport(sixty, three)
	viewport.top = three

	rendered := strings.Split(
		ansi.Strip(v.render(viewport, diffCursor{}, nil)),
		newline,
	)
	requireEqual(
		t,
		len(rendered) == viewport.Height &&
			strings.Contains(rendered[zero], "first.go"),
		true,
		"sticky render = %#v",
		rendered,
	)
	requireEqual(
		t,
		strings.Contains(rendered[one], "first two") &&
			strings.Contains(rendered[two], "first three"),
		true,
		"sticky header covered content rows: %#v",
		rendered,
	)

	secondFileRow := five
	viewport.top = secondFileRow
	rendered = strings.Split(
		ansi.Strip(v.render(viewport, diffCursor{}, nil)),
		newline,
	)
	requireEqual(
		t,
		strings.Count(strings.Join(rendered, newline), fixtureSecondGo), one,
		"file header was duplicated at its natural position: %#v",
		rendered,
	)

	viewport.top = secondFileRow + one
	rendered = strings.Split(
		ansi.Strip(v.render(viewport, diffCursor{}, nil)),
		newline,
	)
	requireEqual(
		t,
		strings.Contains(rendered[zero], fixtureSecondGo), true,
		"sticky header did not change with the file: %#v",
		rendered,
	)
}

func TestKeepVisibleAccountsForStickyFileHeader(t *testing.T) {
	v := newUnifiedView(longPatch(), true)
	cursor, _ := v.last()
	viewport := v.keepVisible(v.newViewport(fifty, four), cursor)
	rendered := ansi.Strip(v.render(viewport, cursor, nil))
	line, _ := v.line(cursor)
	requireEqual(
		t,
		cursor.row < viewport.top+v.contentHeight(viewport) &&
			strings.Contains(rendered, strconv.Itoa(int(line.NewNumber))),
		true,
		"last cursor row is hidden by sticky head"+
			"er: viewport=%#v render=%q",
		viewport,
		rendered,
	)
}

func TestSplitTabsDoNotShiftLineNumbersOrDivider(t *testing.T) {
	p := patch.Patch{
		Files: []patch.File{
			{
				DisplayPath: fixtureFile,
				Hunks: []patch.Hunk{
					{
						Header: hunkMarker,
						Lines: []patch.Line{
							{
								Kind:      patch.Context,
								Text:      "\t\tlong line",
								OldNumber: one,
								NewNumber: one,
							},
						},
					},
				},
			},
		},
	}
	v := newSideBySideView(p, true)
	cursor := mustFirst(t, v)
	rendered := ansi.Strip(renderOne(v, cursor, wideTestWidth, nil))
	requireEqual(
		t,
		!strings.ContainsRune(rendered, '\t') &&
			strings.Index(rendered, "│") == 59 &&
			lipgloss.Width(rendered) == wideTestWidth,
		true,
		"rendered = %q width=%d",
		rendered,
		lipgloss.Width(rendered),
	)
}

func TestHorizontalScrollKeepsUnifiedGutterFixed(t *testing.T) {
	p := longLinePatch()
	v := newUnifiedView(p, true)
	cursor := mustFirst(t, v)
	viewport := v.newViewport(fixtureNumber37, one)
	viewport = v.keepVisible(viewport, cursor)
	before := ansi.Strip(v.render(viewport, cursor, nil))
	viewport = v.scrollHorizontal(viewport, four)
	after := ansi.Strip(v.render(viewport, cursor, nil))
	requireEqual(
		t,
		viewport.LeftColumn == four &&
			(before[:14] == after[:14] &&
				strings.Contains(after[14:], "efghij")),
		true,
		"before=%q after=%q viewport=%#v",
		before,
		after,
		viewport,
	)
}

func TestHorizontalScrollKeepsSplitGuttersAndDividerFixed(t *testing.T) {
	v := newSideBySideView(longLinePatch(), true)
	cursor := mustFirst(t, v)
	viewport := v.newViewport(wideTestWidth, one)
	viewport = v.keepVisible(viewport, cursor)
	viewport = v.scrollHorizontal(viewport, eight)
	rendered := ansi.Strip(v.render(viewport, cursor, nil))
	requireEqual(
		t,
		rendered[63:69] == "    1 " &&
			(rendered[:six] == "    1 " &&
				strings.Index(rendered, "│") == 59),
		true,
		"gutters moved: %q",
		rendered,
	)
	requireEqual(
		t,
		strings.Contains(rendered[six:59], "ghijkl") &&
			strings.Contains(rendered[69:], "ghijkl"),
		true,
		"offset differs: %q",
		rendered,
	)
}

func TestHorizontalScrollStartAndEndClamp(t *testing.T) {
	v := newUnifiedView(longLinePatch(), true)
	viewport := v.newViewport(fixtureNumber37, one)
	viewport = v.scrollHorizontal(viewport, int(^uint(zero)>>one))
	requireEqual(
		t,
		viewport.LeftColumn != zero,
		true,
		fixtureV,
		"end did not scroll",
	)
	viewport = v.scrollHorizontal(viewport, -int(^uint(zero)>>one))
	requireEqual(
		t,
		viewport.LeftColumn, zero,
		"start = %d",
		viewport.LeftColumn,
	)
}

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
	rendered := renderTarget(v, removed, first, fixtureNumber72, &selection)
	requireEqual(
		t,
		!strings.Contains(rendered, "\x1b[1m"), true,
		"selection is bold: %q",
		rendered,
	)
	assertStyledThroughColumn(
		t,
		rendered,
		fixtureNumber72,
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
								OldNumber: two,
							},
							{
								Kind:      patch.Addition,
								Text:      "new()",
								NewNumber: two,
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
	}{{newUnifiedView, fixtureNumber37}, {newSideBySideView, wideTestWidth}} {
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
	viewport := v.newViewport(width, one)
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
						Header: hunkMarker,

						Lines: []patch.Line{
							{
								Kind: patch.Context,

								Text: "abcdefghijklmnopqrstuvwxyz0123456789" +
									"ABCDEFGHIJKLMNOPQRSTUVWXYZ",

								OldNumber: one,

								NewNumber: one,
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
	initialRendered string,
	width int,
	expected sgrExpectation,
) {
	rendered := initialRendered

	t.Helper()
	state := sgrState{}
	column := zero
	for len(rendered) > zero {
		location := sgrPattern.FindStringSubmatchIndex(rendered)
		if location == nil {
			column = assertStyledText(t, rendered, column, state, expected)
			break
		}
		column = assertStyledText(
			t,
			rendered[:location[zero]],
			column,
			state,
			expected,
		)
		state.apply(rendered[location[two]:location[three]])
		rendered = rendered[location[one]:]
	}
	requireEqual(
		t,
		column, width,
		"styled columns = %d, want %d",
		column,
		width,
	)
}
func (s *sgrState) apply(initialParameters string) {
	parameters := initialParameters

	if parameters == "" {
		parameters = "0"
	}
	values := strings.Split(parameters, ";")
	for index := zero; index < len(values); index++ {
		value, _ := strconv.Atoi(values[index])
		switch value {
		case zero:
			*s = sgrState{}
		case seven:
			s.reverse = true
		case fixtureNumber27:
			s.reverse = false
		case sgrDefaultBackground:
			s.background = ""
		case sgrExtendedBackground:
			index = s.applyBackground(values, index)
		default:
			// Other inputs leave the current state unchanged.
		}
	}
}
func assertSGRState(
	t *testing.T,
	column int,
	state sgrState,
	expected sgrExpectation,
) {
	t.Helper()
	requireEqual(
		t,
		state.background == expected.background &&
			state.reverse == expected.reverse,
		true,
		"column %d state=%#v want=%#v",
		column,
		state,
		expected,
	)
}

func (s *sgrState) applyBackground(values []string, index int) int {
	if index+four < len(values) && values[index+one] == "2" {
		s.background = strings.Join(values[index+two:index+five], ";")
		return index + four
	}
	return index
}

func assertStyledText(
	t *testing.T,
	text string,
	start int,
	state sgrState,
	expected sgrExpectation,
) int {
	column := start
	for range text {
		column++
		assertSGRState(t, column, state, expected)
	}
	return column
}

func unequalChangePatch() patch.Patch {
	return patch.Patch{
		Files: []patch.File{
			{
				DisplayPath: fixtureFile,
				Hunks: []patch.Hunk{
					{
						Header: fixtureOne,
						Lines: []patch.Line{
							{
								Kind:      patch.Deletion,
								Text:      "d1",
								OldNumber: one,
							},
							{
								Kind:      patch.Deletion,
								Text:      "d2",
								OldNumber: two,
							},
							{
								Kind:      patch.Addition,
								Text:      "a1",
								NewNumber: one,
							},
							{
								Kind:      patch.Addition,
								Text:      "a2",
								NewNumber: two,
							},
							{
								Kind:      patch.Addition,
								Text:      "a3",
								NewNumber: three,
							},
							{
								Kind:      patch.Context,
								Text:      "c",
								OldNumber: three,
								NewNumber: four,
							},
						},
					},

					{
						Header: fixtureTwo,
						Lines: []patch.Line{
							{
								Kind:      patch.Addition,
								Text:      "separate",
								NewNumber: five,
							},
						},
					},
				},
			},
		},
	}
}

func stickyHeaderPatch() patch.Patch {
	return patch.Patch{
		Files: []patch.File{
			{
				DisplayPath: "first.go",
				Hunks: []patch.Hunk{
					{
						Header: hunkMarker,
						Lines: []patch.Line{
							{
								Kind:      patch.Context,
								Text:      "first one",
								OldNumber: one,
								NewNumber: one,
							},

							{
								Kind:      patch.Context,
								Text:      "first two",
								OldNumber: two,
								NewNumber: two,
							},

							{
								Kind:      patch.Context,
								Text:      "first three",
								OldNumber: three,
								NewNumber: three,
							},
						},
					},
				},
			},

			{
				DisplayPath: fixtureSecondGo,
				Hunks: []patch.Hunk{
					{
						Header: hunkMarker,
						Lines: []patch.Line{
							{
								Kind:      patch.Context,
								Text:      "second one",
								OldNumber: one,
								NewNumber: one,
							},
						},
					},
				},
			},
		},
	}
}
