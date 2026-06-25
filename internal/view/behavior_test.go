package view

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
	p := patch.Patch{Files: []patch.File{{DisplayPath: "file", Hunks: []patch.Hunk{
		{Header: "one", Lines: []patch.Line{{Kind: patch.Deletion, Text: "d1", OldNumber: 1}, {Kind: patch.Deletion, Text: "d2", OldNumber: 2}, {Kind: patch.Addition, Text: "a1", NewNumber: 1}, {Kind: patch.Addition, Text: "a2", NewNumber: 2}, {Kind: patch.Addition, Text: "a3", NewNumber: 3}, {Kind: patch.Context, Text: "c", OldNumber: 3, NewNumber: 4}}},
		{Header: "two", Lines: []patch.Line{{Kind: patch.Addition, Text: "separate", NewNumber: 5}}},
	}}}}
	v := NewSideBySideView(p, true).(*diffView)
	var code []entry
	for _, current := range v.rows {
		if current.kind == lineRow {
			code = append(code, current)
		}
	}
	if len(code) != 5 {
		t.Fatalf("visual code entries = %d", len(code))
	}
	if code[0].leftLine != 0 || code[0].rightLine != 2 || code[1].leftLine != 1 || code[1].rightLine != 3 || code[2].leftLine != -1 || code[2].rightLine != 4 {
		t.Fatalf("pairing = %#v", code[:3])
	}
	if code[4].hunk == code[3].hunk || code[4].leftLine != -1 {
		t.Fatalf("hunks were paired: %#v", code[4])
	}
}

func TestSplitSelectionOnlyIncludesActivePane(t *testing.T) {
	v := NewSideBySideView(testPatch(), true)
	first := mustFirst(t, v)
	removed, _ := v.Search("removed one", first, Forward)
	added, _ := v.Search("added one", first, Forward)
	left := v.BeginSelection(removed)
	left, ok := v.ExtendSelection(left, removed)
	if !ok || len(v.Lines(left)) != 1 || v.Lines(left)[0].Kind != patch.Deletion {
		t.Fatalf("left lines = %#v", v.Lines(left))
	}
	right := v.BeginSelection(added)
	if len(v.Lines(right)) != 1 || v.Lines(right)[0].Kind != patch.Addition {
		t.Fatalf("right lines = %#v", v.Lines(right))
	}
}

func TestSplitPaneSwitchingFindsRowsAboveAndBelowEmptyTargets(t *testing.T) {
	p := patch.Patch{Files: []patch.File{{DisplayPath: "file", Hunks: []patch.Hunk{{Header: "@@", Lines: []patch.Line{
		{Kind: patch.Addition, Text: "right", NewNumber: 1},
		{Kind: patch.Context, Text: "both", OldNumber: 1, NewNumber: 2},
		{Kind: patch.Deletion, Text: "left", OldNumber: 2},
	}}}}}}
	v := NewSideBySideView(p, true)
	first := mustFirst(t, v)
	left, ok := v.SwitchPane(first, Left)
	if !ok {
		t.Fatal("did not find later left pane")
	}
	line, _ := v.Line(left)
	if line.Text != "both" {
		t.Fatalf("later left line = %q", line.Text)
	}
	last, _ := v.Last()
	right, ok := v.SwitchPane(last, Right)
	if !ok {
		t.Fatal("did not find earlier right pane")
	}
	line, _ = v.Line(right)
	if line.Text != "both" {
		t.Fatalf("earlier right line = %q", line.Text)
	}
}

func TestSplitPaneSwitchDoesNothingWhenTargetPaneIsEmpty(t *testing.T) {
	p := patch.Patch{Files: []patch.File{{DisplayPath: "file", Hunks: []patch.Hunk{{Header: "@@", Lines: []patch.Line{{Kind: patch.Addition, Text: "one", NewNumber: 1}, {Kind: patch.Addition, Text: "two", NewNumber: 2}}}}}}}
	v := NewSideBySideView(p, true)
	cursor := mustFirst(t, v)
	if _, ok := v.SwitchPane(cursor, Left); ok {
		t.Fatal("switched to empty pane")
	}
}

func TestSplitVerticalMovementSkipsEmptyActivePane(t *testing.T) {
	p := patch.Patch{Files: []patch.File{{DisplayPath: "file", Hunks: []patch.Hunk{{Header: "@@", Lines: []patch.Line{{Kind: patch.Context, Text: "one", OldNumber: 1, NewNumber: 1}, {Kind: patch.Addition, Text: "right", NewNumber: 2}, {Kind: patch.Context, Text: "two", OldNumber: 2, NewNumber: 3}, {Kind: patch.Deletion, Text: "left", OldNumber: 3}, {Kind: patch.Context, Text: "three", OldNumber: 4, NewNumber: 4}}}}}}}
	v := NewSideBySideView(p, true)
	first := mustFirst(t, v)
	left, _ := v.SwitchPane(first, Left)
	nextLeft, _ := v.Move(left, Forward)
	line, _ := v.Line(nextLeft)
	if line.Text != "two" {
		t.Fatalf("left movement = %q", line.Text)
	}
	rightAtContext, _ := v.SwitchPane(nextLeft, Right)
	nextRight, _ := v.Move(rightAtContext, Forward)
	line, _ = v.Line(nextRight)
	if line.Text != "three" {
		t.Fatalf("right movement = %q", line.Text)
	}
}

func TestSplitVerticalMovementAndHalfPageUseVisualRows(t *testing.T) {
	lines := []patch.Line{
		{Kind: patch.Deletion, Text: "d1", OldNumber: 1}, {Kind: patch.Addition, Text: "a1", NewNumber: 1}, {Kind: patch.Context, Text: "c1", OldNumber: 2, NewNumber: 2},
		{Kind: patch.Deletion, Text: "d2", OldNumber: 3}, {Kind: patch.Addition, Text: "a2", NewNumber: 3}, {Kind: patch.Context, Text: "c2", OldNumber: 4, NewNumber: 4},
		{Kind: patch.Deletion, Text: "d3", OldNumber: 5}, {Kind: patch.Addition, Text: "a3", NewNumber: 5}, {Kind: patch.Context, Text: "c3", OldNumber: 6, NewNumber: 6},
	}
	v := NewSideBySideView(patch.Patch{Files: []patch.File{{DisplayPath: "file", Hunks: []patch.Hunk{{Header: "@@", Lines: lines}}}}}, true)
	cursor := mustFirst(t, v)
	viewport := v.NewViewport(120, 4)
	viewport = v.KeepVisible(viewport, cursor)
	originalTop := viewport.Top
	viewport, moved := v.ScrollHalfPage(viewport, cursor, Forward)
	if viewport.Top.Y <= originalTop.Y || moved.Coordinate.Y <= cursor.Coordinate.Y {
		t.Fatalf("viewport=%#v cursor=%#v", viewport, moved)
	}
	viewport, moved = v.ScrollHalfPage(viewport, moved, Backward)
	if viewport.Top != originalTop || moved.Coordinate != cursor.Coordinate {
		t.Fatalf("round trip viewport=%#v cursor=%#v", viewport, moved)
	}
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
	v := NewUnifiedView(p, true).(*diffView)
	viewport := v.NewViewport(60, 3)
	viewport.Top.Y = 3

	rendered := strings.Split(ansi.Strip(v.Render(viewport, Cursor{}, nil)), "\n")
	if len(rendered) != viewport.Height || !strings.Contains(rendered[0], "first.go") {
		t.Fatalf("sticky render = %#v", rendered)
	}
	if !strings.Contains(rendered[1], "first two") || !strings.Contains(rendered[2], "first three") {
		t.Fatalf("sticky header covered content rows: %#v", rendered)
	}

	secondFileRow := 5
	viewport.Top.Y = secondFileRow
	rendered = strings.Split(ansi.Strip(v.Render(viewport, Cursor{}, nil)), "\n")
	if strings.Count(strings.Join(rendered, "\n"), "second.go") != 1 {
		t.Fatalf("file header was duplicated at its natural position: %#v", rendered)
	}

	viewport.Top.Y = secondFileRow + 1
	rendered = strings.Split(ansi.Strip(v.Render(viewport, Cursor{}, nil)), "\n")
	if !strings.Contains(rendered[0], "second.go") {
		t.Fatalf("sticky header did not change with the file: %#v", rendered)
	}
}

func TestKeepVisibleAccountsForStickyFileHeader(t *testing.T) {
	v := NewUnifiedView(longPatch(), true).(*diffView)
	cursor, _ := v.Last()
	viewport := v.KeepVisible(v.NewViewport(50, 4), cursor)
	rendered := ansi.Strip(v.Render(viewport, cursor, nil))
	line, _ := v.Line(cursor)
	if cursor.Coordinate.Y >= viewport.Top.Y+v.contentHeight(viewport) || !strings.Contains(rendered, strconv.Itoa(int(line.NewNumber))) {
		t.Fatalf("last cursor row is hidden by sticky header: viewport=%#v render=%q", viewport, rendered)
	}
}

func TestSplitTabsDoNotShiftLineNumbersOrDivider(t *testing.T) {
	p := patch.Patch{Files: []patch.File{{DisplayPath: "file", Hunks: []patch.Hunk{{Header: "@@", Lines: []patch.Line{{Kind: patch.Context, Text: "\t\tlong line", OldNumber: 1, NewNumber: 1}}}}}}}
	v := NewSideBySideView(p, true)
	cursor := mustFirst(t, v)
	rendered := ansi.Strip(renderOne(v, cursor, 120, nil))
	if strings.ContainsRune(rendered, '\t') || strings.Index(rendered, "│") != 59 || lipgloss.Width(rendered) != 120 {
		t.Fatalf("rendered = %q width=%d", rendered, lipgloss.Width(rendered))
	}
}

func TestHorizontalScrollKeepsUnifiedGutterFixed(t *testing.T) {
	p := longLinePatch()
	v := NewUnifiedView(p, true)
	cursor := mustFirst(t, v)
	viewport := v.NewViewport(37, 1)
	viewport = v.KeepVisible(viewport, cursor)
	before := ansi.Strip(v.Render(viewport, cursor, nil))
	viewport = v.ScrollHorizontal(viewport, 4)
	after := ansi.Strip(v.Render(viewport, cursor, nil))
	if before[:14] != after[:14] || !strings.Contains(after[14:], "efghij") || viewport.LeftColumn != 4 {
		t.Fatalf("before=%q after=%q viewport=%#v", before, after, viewport)
	}
}

func TestHorizontalScrollKeepsSplitGuttersAndDividerFixed(t *testing.T) {
	v := NewSideBySideView(longLinePatch(), true)
	cursor := mustFirst(t, v)
	viewport := v.NewViewport(120, 1)
	viewport = v.KeepVisible(viewport, cursor)
	viewport = v.ScrollHorizontal(viewport, 8)
	rendered := ansi.Strip(v.Render(viewport, cursor, nil))
	if strings.Index(rendered, "│") != 59 || rendered[:6] != "    1 " || rendered[63:69] != "    1 " {
		t.Fatalf("gutters moved: %q", rendered)
	}
	if !strings.Contains(rendered[6:59], "ghijkl") || !strings.Contains(rendered[69:], "ghijkl") {
		t.Fatalf("offset differs: %q", rendered)
	}
}

func TestHorizontalScrollStartAndEndClamp(t *testing.T) {
	v := NewUnifiedView(longLinePatch(), true)
	viewport := v.NewViewport(37, 1)
	viewport = v.ScrollHorizontal(viewport, int(^uint(0)>>1))
	if viewport.LeftColumn == 0 {
		t.Fatal("end did not scroll")
	}
	viewport = v.ScrollHorizontal(viewport, -int(^uint(0)>>1))
	if viewport.LeftColumn != 0 {
		t.Fatalf("start = %d", viewport.LeftColumn)
	}
}

func TestDiffMarkersUseTerminalColorsAndCursorFillsWidth(t *testing.T) {
	v := NewUnifiedView(testPatch(), true)
	first := mustFirst(t, v)
	added, _ := v.Search("added one", first, Forward)
	removed, _ := v.Search("removed one", first, Forward)
	addedRender := renderTarget(v, added, first, 80, nil)
	removedRender := renderTarget(v, removed, first, 80, nil)
	if !strings.Contains(addedRender, "\x1b[32m+\x1b[m") || !strings.Contains(removedRender, "\x1b[31m-\x1b[m") {
		t.Fatalf("added=%q removed=%q", addedRender, removedRender)
	}
	assertStyledThroughColumn(t, renderOne(v, added, 80, nil), 80, sgrExpectation{reverse: true})
}

func TestSelectionBackgroundKeepsDefaultWeight(t *testing.T) {
	v := NewUnifiedView(testPatch(), false)
	first := mustFirst(t, v)
	removed, _ := v.Search("removed one", first, Forward)
	selection := v.BeginSelection(removed)
	rendered := renderTarget(v, removed, first, 72, &selection)
	if strings.Contains(rendered, "\x1b[1m") {
		t.Fatalf("selection is bold: %q", rendered)
	}
	assertStyledThroughColumn(t, rendered, 72, sgrExpectation{background: "219;234;254"})
}

func TestSyntaxHighlightingSurvivesDiffStyling(t *testing.T) {
	p := patch.Patch{Files: []patch.File{{DisplayPath: "main.go", NewPath: "main.go", OldSource: "package main\nold()\n", NewSource: "package main\nnew()\n", Hunks: []patch.Hunk{{Header: "@@", Lines: []patch.Line{{Kind: patch.Deletion, Text: "old()", OldNumber: 2}, {Kind: patch.Addition, Text: "new()", NewNumber: 2}}}}}}}
	v := NewUnifiedView(p, true)
	first := mustFirst(t, v)
	added, _ := v.Search("new()", first, Forward)
	for _, cursor := range []Cursor{first, added} {
		rendered := renderTarget(v, cursor, Cursor{}, 80, nil)
		if !strings.Contains(rendered, "[38;2;") {
			t.Fatalf("syntax highlighting missing: %q", rendered)
		}
	}
}

func TestRenderedCodeRowsHaveExactTerminalWidth(t *testing.T) {
	for _, test := range []struct {
		constructor func(patch.Patch, bool) View
		width       int
	}{{NewUnifiedView, 37}, {NewSideBySideView, 120}} {
		v := test.constructor(testPatch(), true)
		cursor := mustFirst(t, v)
		for {
			if width := lipgloss.Width(renderOne(v, cursor, test.width, nil)); width != test.width {
				t.Fatalf("width = %d", width)
			}
			next, ok := v.Move(cursor, Forward)
			if !ok {
				break
			}
			cursor = next
		}
	}
}

func TestRenderStyledRowStripsSyntaxBackgroundColors(t *testing.T) {
	value := strings.Join([]string{"\x1b[48;2;255;0;0;38;2;1;2;3mtruecolor", "\x1b[48;5;123;1mindexed", "\x1b[45mstandard", "\x1b[105mbright"}, " ")
	rendered := renderStyledRow(lineStyle(patch.Addition, true), value, 80, false)
	for _, forbidden := range []string{"48;2;255;0;0", "48;5;123", "[45m", "[105m"} {
		if strings.Contains(rendered, forbidden) {
			t.Fatalf("retains %q: %q", forbidden, rendered)
		}
	}
}

func renderOne(v View, cursor Cursor, width int, selection *Selection) string {
	return renderTarget(v, cursor, cursor, width, selection)
}

func renderTarget(v View, target, active Cursor, width int, selection *Selection) string {
	viewport := v.NewViewport(width, 1)
	viewport.Top = target.Coordinate
	return v.Render(viewport, active, selection)
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
		location := sgrPattern.FindStringSubmatchIndex(rendered)
		if location == nil {
			for range rendered {
				column++
				assertSGRState(t, column, state, expected)
			}
			break
		}
		for range rendered[:location[0]] {
			column++
			assertSGRState(t, column, state, expected)
		}
		state.apply(rendered[location[2]:location[3]])
		rendered = rendered[location[1]:]
	}
	if column != width {
		t.Fatalf("styled columns = %d, want %d", column, width)
	}
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
			if index+4 < len(values) && values[index+1] == "2" {
				s.background = strings.Join(values[index+2:index+5], ";")
				index += 4
			}
		}
	}
}
func assertSGRState(t *testing.T, column int, state sgrState, expected sgrExpectation) {
	t.Helper()
	if state.background != expected.background || state.reverse != expected.reverse {
		t.Fatalf("column %d state=%#v want=%#v", column, state, expected)
	}
}
