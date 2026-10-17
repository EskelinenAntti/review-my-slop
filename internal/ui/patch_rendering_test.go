package ui

import (
	"regexp"
	"strconv"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	patches "github.com/eskelinenantti/review-my-slop/internal/patch"
)

func TestDiffscreenSplitPairsUnequalChangeBlocksAndKeepsHunksSeparate(
	t *testing.T,
) {
	p := patches.Patch{
		Files: []patches.File{{DisplayPath: "file", Hunks: []patches.Hunk{
			{
				Header: "one",
				Lines: []patches.Line{
					{Kind: patches.Deletion, Text: "d1", OldNumber: 1},
					{Kind: patches.Deletion, Text: "d2", OldNumber: 2},
					{Kind: patches.Addition, Text: "a1", NewNumber: 1},
					{Kind: patches.Addition, Text: "a2", NewNumber: 2},
					{Kind: patches.Addition, Text: "a3", NewNumber: 3},
					{
						Kind:      patches.Context,
						Text:      "c",
						OldNumber: 3,
						NewNumber: 4,
					},
				},
			},
			{
				Header: "two",
				Lines: []patches.Line{
					{Kind: patches.Addition, Text: "separate", NewNumber: 5},
				},
			},
		}}},
	}
	v := diffscreenTestSplitViewWithTheme(p, true)
	code := diffscreenCodeEntries(v.rows)
	requireEqual(t, len(code), 5, "visual code entries = %d", len(code))
	wants := []entry{
		{leftLine: 0, rightLine: 2},
		{leftLine: 1, rightLine: 3},
		{leftLine: -1, rightLine: 4},
	}
	for index, want := range wants {
		assertDiffscreenSplitPair(t, code[index], want.leftLine, want.rightLine)
	}
	requireEqual(t, code[4].hunk != code[3].hunk, true,
		"hunks were paired: %#v", code[4],
	)
	requireEqual(t, code[4].leftLine, -1, "hunks were paired: %#v", code[4])
}

func assertDiffscreenSplitPair(
	t *testing.T, got entry, left, right int,
) {
	t.Helper()
	requireEqual(t, got.leftLine, left, "pairing = %#v", got)
	requireEqual(t, got.rightLine, right, "pairing = %#v", got)
}

func diffscreenCodeEntries(rows []entry) []entry {
	var code []entry
	for _, current := range rows {
		if current.kind == lineRow {
			code = append(code, current)
		}
	}
	return code
}

func TestDiffscreenSplitSelectionOnlyIncludesActivePane(t *testing.T) {
	v := newDiffscreenTestView(
		diffscreenTwoFileSelectionPatch(),
		Options{SideBySide: true, Dark: true},
	)
	first, ok := diffscreenSearchCandidateCursor(v.view)
	requireEqual(t, ok, true, "%s", "no first cursor")
	removed, ok := findSearchCursor(v.view, "removed one", first)
	requireEqual(t, ok, true, "%s", "removed line not found")
	added, ok := findSearchCursor(v.view, "added one", first)
	requireEqual(t, ok, true, "%s", "added line not found")

	v.cursor = removed
	v.ToggleSelection()
	_, leftLines, ok := v.Selected()
	requireEqual(t, ok, true, "left selection = %#v", leftLines)
	requireEqual(t, len(leftLines), 1, "left lines = %#v", leftLines)
	requireEqual(t, leftLines[0].Kind, patches.Deletion,
		"left lines = %#v", leftLines,
	)

	v.ClearSelection()
	v.cursor = added
	v.ToggleSelection()
	_, rightLines, ok := v.Selected()
	requireEqual(t, ok, true, "right selection = %#v", rightLines)
	requireEqual(t, len(rightLines), 1, "right lines = %#v", rightLines)
	requireEqual(t, rightLines[0].Kind, patches.Addition,
		"right lines = %#v", rightLines,
	)
}

func TestDiffscreenSplitPaneSwitchingFindsRowsAboveAndBelowEmptyTargets(
	t *testing.T,
) {
	p := patches.Patch{
		Files: []patches.File{
			{
				DisplayPath: "file",
				Hunks: []patches.Hunk{{Header: "@@", Lines: []patches.Line{
					{Kind: patches.Addition, Text: "right", NewNumber: 1},
					{
						Kind:      patches.Context,
						Text:      "both",
						OldNumber: 1,
						NewNumber: 2,
					},
					{Kind: patches.Deletion, Text: "left", OldNumber: 2},
				}}},
			},
		},
	}
	screen := newDiffscreenTestView(
		p, Options{SideBySide: true, Dark: true},
	)
	screen.Resize(120, 10)
	view := screen.view
	first, ok := diffscreenSearchCandidateCursor(view)
	requireEqual(t, ok, true, "%s", "no first cursor")
	leftCursor, ok := closestCursorInPane(left, view, first)
	requireEqual(t, ok, true, "%s", "did not find later left pane")
	line, ok := lineForCursor(leftCursor, view)
	requireEqual(t, ok, true, "left cursor=%#v", leftCursor)
	requireEqual(t, line.Text, "both", "later left line = %q", line.Text)

	last, ok := lastCursor(view)
	requireEqual(t, ok, true, "%s", "no last cursor")
	rightCursor, ok := closestCursorInPane(right, view, last)
	requireEqual(t, ok, true, "%s", "did not find earlier right pane")
	line, ok = lineForCursor(rightCursor, view)
	requireEqual(t, ok, true, "right cursor=%#v", rightCursor)
	requireEqual(t, line.Text, "both", "earlier right line = %q", line.Text)
}

func TestDiffscreenSplitPaneSwitchDoesNothingWhenTargetPaneIsEmpty(
	t *testing.T,
) {
	p := patches.Patch{
		Files: []patches.File{
			{
				DisplayPath: "file",
				Hunks: []patches.Hunk{
					{
						Header: "@@",
						Lines: []patches.Line{
							{Kind: patches.Addition, Text: "one", NewNumber: 1},
							{Kind: patches.Addition, Text: "two", NewNumber: 2},
						},
					},
				},
			},
		},
	}
	screen := newDiffscreenTestView(
		p, Options{SideBySide: true, Dark: true},
	)
	screen.Resize(120, 10)
	cursor, ok := diffscreenSearchCandidateCursor(screen.view)
	requireEqual(t, ok, true, "%s", "no first cursor")
	screen.cursor = cursor
	switchPane(screen, left)
	requireEqual(t, screen.cursor, cursor, "empty pane changed cursor")
}

func TestDiffscreenSplitVerticalMovementSkipsEmptyActivePane(t *testing.T) {
	p := patches.Patch{
		Files: []patches.File{
			{
				DisplayPath: "file",
				Hunks: []patches.Hunk{
					{
						Header: "@@",
						Lines: []patches.Line{
							{
								Kind:      patches.Context,
								Text:      "one",
								OldNumber: 1,
								NewNumber: 1,
							},
							{
								Kind:      patches.Addition,
								Text:      "right",
								NewNumber: 2,
							},
							{
								Kind:      patches.Context,
								Text:      "two",
								OldNumber: 2,
								NewNumber: 3,
							},
							{
								Kind:      patches.Deletion,
								Text:      "left",
								OldNumber: 3,
							},
							{
								Kind:      patches.Context,
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
	screen := newDiffscreenTestView(
		p, Options{SideBySide: true, Dark: true},
	)
	screen.Resize(120, 10)
	first, ok := diffscreenSearchCandidateCursor(screen.view)
	requireEqual(t, ok, true, "%s", "no first cursor")
	screen.cursor = first
	switchPane(screen, left)
	screen.Move(NextLine)
	_, line, ok := screen.Current()
	requireEqual(t, ok, true, "left cursor=%#v", screen.cursor)
	requireEqual(t, line.Text, "two", "left movement = %q", line.Text)

	switchPane(screen, right)
	screen.Move(NextLine)
	_, line, ok = screen.Current()
	requireEqual(t, ok, true, "right cursor=%#v", screen.cursor)
	requireEqual(t, line.Text, "three", "right movement = %q", line.Text)
}

func TestDiffscreenSplitVerticalMovementAndHalfPageUseVisualRows(t *testing.T) {
	lines := []patches.Line{
		{
			Kind:      patches.Deletion,
			Text:      "d1",
			OldNumber: 1,
		}, {
			Kind:      patches.Addition,
			Text:      "a1",
			NewNumber: 1,
		}, {Kind: patches.Context, Text: "c1", OldNumber: 2, NewNumber: 2},
		{
			Kind:      patches.Deletion,
			Text:      "d2",
			OldNumber: 3,
		}, {
			Kind:      patches.Addition,
			Text:      "a2",
			NewNumber: 3,
		}, {Kind: patches.Context, Text: "c2", OldNumber: 4, NewNumber: 4},
		{
			Kind:      patches.Deletion,
			Text:      "d3",
			OldNumber: 5,
		}, {
			Kind:      patches.Addition,
			Text:      "a3",
			NewNumber: 5,
		}, {Kind: patches.Context, Text: "c3", OldNumber: 6, NewNumber: 6},
	}
	patch := patches.Patch{Files: []patches.File{{
		DisplayPath: "file",
		Hunks:       []patches.Hunk{{Header: "@@", Lines: lines}},
	}}}
	screen := newDiffscreenTestView(
		patch,
		Options{SideBySide: true, Dark: true},
	)
	screen.Resize(120, 7)
	start, ok := firstCursor(screen.view)
	requireEqual(t, ok, true, "%s", "no initial cursor")
	screen.cursor = start
	screen.viewport = cursorVisibleViewport(
		start, screen.view, screen.viewport,
	)
	originalTop := screen.viewport.top
	screen.Move(NextPage)
	requireEqual(t, screen.viewport.top > originalTop, true,
		"page down viewport=%#v cursor=%#v", screen.viewport, screen.cursor,
	)
	requireEqual(t, screen.cursor.row > start.row, true,
		"page down viewport=%#v cursor=%#v", screen.viewport, screen.cursor,
	)
	moved := screen.cursor
	screen.Move(PreviousPage)
	requireEqual(t, screen.viewport.top, originalTop,
		"round trip viewport=%#v cursor=%#v", screen.viewport, screen.cursor,
	)
	requireEqual(t, screen.cursor.row, start.row,
		"round trip from=%#v moved=%#v cursor=%#v", start, moved, screen.cursor,
	)
}

func diffscreenStickyHeaderPatch() patches.Patch {
	return patches.Patch{Files: []patches.File{{
		DisplayPath: "first.go",
		Hunks: []patches.Hunk{
			{
				Header: "@@",
				Lines: []patches.Line{
					{
						Kind:      patches.Context,
						Text:      "first one",
						OldNumber: 1,
						NewNumber: 1,
					},
					{
						Kind:      patches.Context,
						Text:      "first two",
						OldNumber: 2,
						NewNumber: 2,
					},
					{
						Kind:      patches.Context,
						Text:      "first three",
						OldNumber: 3,
						NewNumber: 3,
					},
				},
			},
		},
	}, {DisplayPath: "second.go", Hunks: []patches.Hunk{{
		Header: "@@",
		Lines: []patches.Line{
			{
				Kind:      patches.Context,
				Text:      "second one",
				OldNumber: 1,
				NewNumber: 1,
			},
		},
	}}}}}
}

func TestDiffscreenFileHeaderSticksWithoutCoveringDiffRows(t *testing.T) {
	p := diffscreenStickyHeaderPatch()
	v := diffscreenTestViewWithTheme(p, true)
	viewport := initialViewport(v, 60, 3)
	viewport.top = 3

	rendered := strings.Split(
		ansi.Strip(
			renderPatch(v, viewport, diffCursor{}, nil),
		),
		"\n",
	)
	requireEqual(t, len(rendered), viewport.Height,
		"sticky render = %#v", rendered,
	)
	requireEqual(t, strings.Contains(rendered[0], "first.go"), true,
		"sticky render = %#v", rendered,
	)
	requireEqual(t, strings.Contains(rendered[1], "first two"), true,
		"sticky header covered content rows: %#v", rendered,
	)
	requireEqual(t, strings.Contains(rendered[2], "first three"), true,
		"sticky header covered content rows: %#v", rendered,
	)

	secondFileRow := 5
	viewport.top = secondFileRow
	rendered = strings.Split(
		ansi.Strip(
			renderPatch(v, viewport, diffCursor{}, nil),
		),
		"\n",
	)
	requireEqual(t, strings.Count(strings.Join(rendered, "\n"), "second.go"), 1,
		"file header was duplicated at its natural position: %#v", rendered)

	viewport.top = secondFileRow + 1
	rendered = strings.Split(
		ansi.Strip(
			renderPatch(v, viewport, diffCursor{}, nil),
		),
		"\n",
	)
	requireEqual(t, strings.Contains(rendered[0], "second.go"), true,
		"sticky header did not change with the file: %#v", rendered,
	)
}

func TestDiffscreenKeepVisibleAccountsForStickyFileHeader(t *testing.T) {
	view := diffscreenTestViewWithTheme(diffscreenLongContextPatch(), true)
	cursor, ok := lastCursor(view)
	requireEqual(t, ok, true, "%s", "no last cursor")
	viewport := initialViewport(view, 50, 4)
	viewport = cursorVisibleViewport(cursor, view, viewport)
	rendered := ansi.Strip(renderPatch(view, viewport, cursor, nil))
	_, line, ok := patchLineAtCursor(view, cursor)
	requireEqual(t, ok, true, "last cursor=%#v", cursor)
	requireEqual(t, cursorVisible(view, viewport, cursor), true,
		"last cursor row is hidden by sticky header: viewport=%#v render=%q",
		viewport, rendered,
	)
	requireEqual(
		t, strings.Contains(rendered, strconv.Itoa(int(line.NewNumber))), true,
		"last cursor row is hidden by sticky header: viewport=%#v render=%q",
		viewport, rendered,
	)
}

func horizontalScrollTestPatch() patches.Patch {
	return patches.Patch{Files: []patches.File{{
		DisplayPath: "long",
		Hunks: []patches.Hunk{{
			Header: "@@",
			Lines: []patches.Line{{
				Kind: patches.Context,
				Text: "abcdefghijklmnopqrstuvwxyz0123456789" +
					"ABCDEFGHIJKLMNOPQRSTUVWXYZ",
				OldNumber: 1, NewNumber: 1,
			}},
		}},
	}}}
}

func renderedDiffscreen(screen *Diff) string {
	return ansi.Strip(renderPatch(
		screen.view, screen.viewport, screen.cursor, screen.selection,
	))
}

func TestDiffscreenHorizontalScrollKeepsUnifiedGutterFixed(t *testing.T) {
	screen := newDiffscreenTestView(
		horizontalScrollTestPatch(), Options{Dark: true},
	)
	screen.Resize(37, 4)
	before := renderedDiffscreen(screen)
	screen.ScrollHorizontal(4)
	after := renderedDiffscreen(screen)
	requireEqual(t, before[:14], after[:14],
		"before=%q after=%q viewport=%#v", before, after, screen.viewport,
	)
	requireEqual(t, strings.Contains(after[14:], "efghij"), true,
		"before=%q after=%q viewport=%#v", before, after, screen.viewport,
	)
	requireEqual(t, screen.viewport.LeftColumn, 4,
		"before=%q after=%q viewport=%#v", before, after, screen.viewport,
	)
}

func TestDiffscreenHorizontalScrollKeepsSplitGuttersAndDividerFixed(
	t *testing.T,
) {
	screen := newDiffscreenTestView(
		horizontalScrollTestPatch(),
		Options{SideBySide: true, Dark: true},
	)
	screen.Resize(120, 4)
	screen.ScrollHorizontal(8)
	rendered := renderedDiffscreen(screen)
	requireEqual(t, strings.Index(rendered, "│"), 59,
		"gutters moved: %q", rendered,
	)
	requireEqual(t, rendered[:6], "    1 ", "gutters moved: %q", rendered)
	requireEqual(t, rendered[63:69], "    1 ", "gutters moved: %q", rendered)
	requireEqual(t, strings.Contains(rendered[6:59], "ghijkl"), true,
		"offset differs: %q", rendered,
	)
	requireEqual(t, strings.Contains(rendered[69:], "ghijkl"), true,
		"offset differs: %q", rendered,
	)
}

func TestDiffscreenHorizontalScrollStartAndEndClamp(t *testing.T) {
	screen := newDiffscreenTestView(
		horizontalScrollTestPatch(), Options{Dark: true},
	)
	screen.Resize(37, 4)
	maximum := int(^uint(0) >> 1)
	screen.ScrollHorizontal(maximum)
	requireEqual(t, screen.viewport.LeftColumn != 0, true, "%s",
		"end did not scroll",
	)
	screen.ScrollHorizontal(-maximum)
	requireEqual(t, screen.viewport.LeftColumn, 0, "start = %d",
		screen.viewport.LeftColumn,
	)
}

func diffMarkerTestPatch() patches.Patch {
	return patches.Patch{
		Root: "/repo",
		Files: []patches.File{
			{
				DisplayPath: "first.go",
				OldPath:     "first.go",
				NewPath:     "first.go",
				Hunks: []patches.Hunk{{
					Header: "@@ -1,3 +1,3 @@",
					Lines: []patches.Line{
						{
							Kind: patches.Context, Text: "before",
							OldNumber: 1, NewNumber: 1,
						},
						{
							Kind: patches.Deletion, Text: "removed one",
							OldNumber: 2,
						},
						{
							Kind: patches.Deletion, Text: "removed two",
							OldNumber: 3,
						},
						{
							Kind:      patches.Addition,
							Text:      "added one",
							NewNumber: 2,
						},
						{
							Kind: patches.Context, Text: "after",
							OldNumber: 4, NewNumber: 3,
						},
					},
				}},
			},
			{
				DisplayPath: "second.go",
				OldPath:     "second.go",
				NewPath:     "second.go",
				Hunks: []patches.Hunk{{
					Header: "@@ -1 +1 @@",
					Lines: []patches.Line{
						{Kind: patches.Addition, Text: "other", NewNumber: 1},
					},
				}},
			},
		},
	}
}

func TestDiffscreenDiffMarkersUseTerminalColorsAndCursorFillsWidth(
	t *testing.T,
) {
	view := diffscreenTestViewWithTheme(diffMarkerTestPatch(), true)
	first, ok := diffscreenSearchCandidateCursor(view)
	requireEqual(t, ok, true, "%s", "no first cursor")
	added, ok := findSearchCursor(view, "added one", first)
	requireEqual(t, ok, true, "%s", "added line not found")
	removed, ok := findSearchCursor(view, "removed one", first)
	requireEqual(t, ok, true, "%s", "removed line not found")
	addedRender := diffscreenRenderAtCursor(view, added, first, 80, nil)
	removedRender := diffscreenRenderAtCursor(view, removed, first, 80, nil)
	requireEqual(t, strings.Contains(addedRender, "\x1b[32m+\x1b[m"), true,
		"added=%q removed=%q", addedRender, removedRender,
	)
	requireEqual(t, strings.Contains(removedRender, "\x1b[31m-\x1b[m"), true,
		"added=%q removed=%q", addedRender, removedRender,
	)
	rendered := diffscreenRenderAtCursor(view, added, added, 80, nil)
	assertDiffscreenRenderedSGR(
		t, rendered, diffscreenSgrExpectation{reverse: true}, 80,
	)
}

func TestDiffscreenSelectionBackgroundKeepsDefaultWeight(t *testing.T) {
	view := diffscreenTestViewWithTheme(diffMarkerTestPatch(), false)
	first, ok := diffscreenSearchCandidateCursor(view)
	requireEqual(t, ok, true, "%s", "no first cursor")
	removed, ok := findSearchCursor(view, "removed one", first)
	requireEqual(t, ok, true, "%s", "removed line not found")
	selection := diffSelection{First: removed, Last: removed}
	rendered := diffscreenRenderAtCursor(view, removed, first, 72, &selection)
	requireEqual(t, strings.Contains(rendered, "\x1b[1m"), false,
		"selection is bold: %q", rendered,
	)
	assertDiffscreenRenderedSGR(
		t, rendered, diffscreenSgrExpectation{background: "219;234;254"}, 72,
	)
}

func assertDiffscreenRenderedSGR(
	t *testing.T, rendered string, expected diffscreenSgrExpectation, width int,
) {
	t.Helper()
	state := diffscreenSgrState{}
	column := 0
	for len(rendered) > 0 {
		location := diffscreenSgrPattern.FindStringSubmatchIndex(rendered)
		if location == nil {
			column = assertDiffscreenSGRText(
				t, rendered, column, state, expected,
			)
			break
		}
		column = assertDiffscreenSGRText(
			t, rendered[:location[0]], column, state, expected,
		)
		applyDiffscreenSGR(&state, rendered[location[2]:location[3]])
		rendered = rendered[location[1]:]
	}
	requireEqual(t, column, width, "styled columns = %d, want %d",
		column, width,
	)
}

func assertDiffscreenSGRText(
	t *testing.T, text string, column int,
	state diffscreenSgrState, expected diffscreenSgrExpectation,
) int {
	t.Helper()
	for range text {
		column++
		requireEqual(t, state.background, expected.background,
			"column %d state=%#v want=%#v", column, state, expected,
		)
		requireEqual(t, state.reverse, expected.reverse,
			"column %d state=%#v want=%#v", column, state, expected,
		)
	}
	return column
}

func applyDiffscreenSGR(state *diffscreenSgrState, parameters string) {
	if parameters == "" {
		parameters = "0"
	}
	values := strings.Split(parameters, ";")
	for index := 0; index < len(values); index++ {
		value, _ := strconv.Atoi(values[index])
		switch value {
		case 0:
			*state = diffscreenSgrState{}
		case 7:
			state.reverse = true
		case 27:
			state.reverse = false
		case 49:
			state.background = ""
		case 48:
			index = applyDiffscreenSGRBackground(state, values, index)
		default:
		}
	}
}

func applyDiffscreenSGRBackground(
	state *diffscreenSgrState, values []string, index int,
) int {
	if index+4 >= len(values) || values[index+1] != "2" {
		return index
	}
	state.background = strings.Join(values[index+2:index+5], ";")
	return index + 4
}

func TestDiffscreenSyntaxHighlightingSurvivesDiffStyling(t *testing.T) {
	p := patches.Patch{
		Files: []patches.File{
			{
				DisplayPath: "main.go",
				OldPath:     "main.go",
				NewPath:     "main.go",
				OldSource:   "package main\nold()\n",
				NewSource:   "package main\nnew()\n",
				Hunks: []patches.Hunk{
					{
						Header: "@@",
						Lines: []patches.Line{
							{
								Kind:      patches.Deletion,
								Text:      "old()",
								OldNumber: 2,
							},
							{
								Kind:      patches.Addition,
								Text:      "new()",
								NewNumber: 2,
							},
						},
					},
				},
			},
		},
	}
	v := diffscreenTestViewWithTheme(p, true)
	first := func() diffCursor {
		t.Helper()
		cursor, ok := diffscreenSearchCandidateCursor(v)
		requireEqual(t, ok, true, "%s", "no first cursor")
		return cursor
	}()
	added, ok := findSearchCursor(v, "new()", first)
	requireEqual(t, ok, true, "%s", "added line not found")
	for _, cursor := range []diffCursor{first, added} {
		rendered := diffscreenRenderAtCursor(
			v,
			cursor,
			diffCursor{},
			80,
			nil,
		)
		requireEqual(t, strings.Contains(rendered, "[38;2;"), true,
			"syntax highlighting missing: %q", rendered,
		)
	}
}

func TestDiffscreenRenderedCodeRowsHaveExactTerminalWidth(t *testing.T) {
	for _, test := range []struct {
		constructor func(patches.Patch, bool) *diffView
		width       int
	}{
		{
			diffscreenTestViewWithTheme,
			37,
		},
		{
			diffscreenTestSplitViewWithTheme,
			120,
		},
	} {
		v := test.constructor(diffscreenTwoFileSelectionPatch(), true)
		cursor := func() diffCursor {
			t.Helper()
			cursor, ok := diffscreenSearchCandidateCursor(v)
			requireEqual(t, ok, true, "%s", "no first cursor")
			return cursor
		}()
		for {
			rendered := diffscreenRenderAtCursor(
				v, cursor, cursor, test.width, nil,
			)
			width := lipgloss.Width(rendered)
			requireEqual(t, width, test.width, "width=%d want=%d",
				width, test.width,
			)
			next, ok := nextCursor(v, cursor.pane, cursor.row)
			if !ok {
				break
			}
			cursor = next
		}
	}
}

func TestDiffscreenRenderStyledRowStripsSyntaxBackgroundColors(t *testing.T) {
	value := strings.Join([]string{
		"\x1b[48;2;255;0;0;38;2;1;2;3mtruecolor",
		"\x1b[48;5;123;1mindexed",
		"\x1b[45mstandard",
		"\x1b[105mbright",
	}, " ")
	p := patches.Patch{Files: []patches.File{{
		DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go",
		Hunks: []patches.Hunk{{Lines: []patches.Line{{
			Kind: patches.Addition, Text: value,
		}}}},
	}}}
	v := diffscreenTestViewWithTheme(p, true)
	cursor, ok := diffscreenSearchCandidateCursor(v)
	requireEqual(t, ok, true, "%s", "no first cursor")
	rendered := diffscreenRenderAtCursor(
		v, cursor, diffCursor{}, 80, nil,
	)
	for _, forbidden := range []string{
		"48;2;255;0;0",
		"48;5;123",
		"[45m",
		"[105m",
	} {
		requireEqual(t, !strings.Contains(rendered, forbidden), true,
			"retains %q: %q", forbidden, rendered,
		)
	}
}

type diffscreenSgrExpectation struct {
	background string
	reverse    bool
}

type diffscreenSgrState struct {
	background string
	reverse    bool
}

var diffscreenSgrPattern = regexp.MustCompile(`\x1b\[([0-9;]*)m`)

func TestDiffscreenSplitTabsDoNotShiftLineNumbersOrDivider(t *testing.T) {
	patch := patches.Patch{Files: []patches.File{{
		DisplayPath: "file",
		Hunks: []patches.Hunk{{
			Header: "@@",
			Lines: []patches.Line{{
				Kind: patches.Context, Text: "\t\tlong line",
				OldNumber: 1, NewNumber: 1,
			}},
		}},
	}}}
	view := diffscreenTestSplitViewWithTheme(patch, true)
	cursor, ok := diffscreenSearchCandidateCursor(view)
	requireEqual(t, ok, true, "%s", "no first cursor")
	rendered := ansi.Strip(diffscreenRenderAtCursor(
		view, cursor, cursor, 120, nil,
	))
	requireEqual(t, strings.ContainsRune(rendered, '\t'), false,
		"rendered = %q", rendered,
	)
	requireEqual(t, strings.Index(rendered, "│"), 59,
		"rendered = %q", rendered,
	)
	requireEqual(t, lipgloss.Width(rendered), 120,
		"rendered = %q width=%d", rendered, lipgloss.Width(rendered),
	)
}
