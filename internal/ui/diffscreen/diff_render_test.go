package diffscreen

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestRendererLineStylePrecedence(t *testing.T) {
	styles := newLineStyles(true)
	candidate := diffCursor{row: 1, pane: right}
	selection := beginSelection(candidate)
	for _, tc := range []struct {
		name      string
		kind      patch.LineKind
		cursor    diffCursor
		selection *diffSelection
		style     lipgloss.Style
		colors    foregroundPolicy
	}{
		{
			"context", patch.Context, diffCursor{}, nil,
			contextStyle, preserveForeground,
		},
		{
			"addition", patch.Addition, diffCursor{}, nil,
			styles.added, preserveForeground,
		},
		{
			"deletion", patch.Deletion, diffCursor{}, nil,
			styles.removed, preserveForeground,
		},
		{
			"selection overrides addition", patch.Addition,
			diffCursor{}, &selection, styles.selection, discardForeground,
		},
		{
			"selection overrides deletion", patch.Deletion,
			diffCursor{}, &selection, styles.selection, discardForeground,
		},
		{
			"cursor overrides addition", patch.Addition, candidate, nil,
			cursorStyle, discardForeground,
		},
		{
			"cursor overrides selection", patch.Deletion, candidate,
			&selection, cursorStyle, discardForeground,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			renderer := diffRenderer{
				view: &diffView{styles: styles}, cursor: tc.cursor,
				selection: tc.selection,
			}
			style, colors := renderer.lineStyle(tc.kind, candidate)
			if !reflect.DeepEqual(style, tc.style) {
				t.Fatal("unexpected line style")
			}
			testutil.RequireEqual(t, colors, tc.colors,
				"unexpected foreground policy")
		})
	}
}

func TestRenderProjectedLineNumbers(t *testing.T) {
	p := patchWithLines([]patch.Line{
		{Kind: patch.Deletion, Text: "old", OldNumber: 10},
		{Kind: patch.Addition, Text: "new", NewNumber: 20},
		{Kind: patch.Addition, Text: "extra", NewNumber: 21},
		{Kind: patch.Context, Text: "shared", OldNumber: 11, NewNumber: 22},
	})
	for _, tc := range []struct {
		name  string
		split bool
		line  int
		pane  diffPane
		want  string
	}{
		{"unified deletion", false, 0, right, "10 - old"},
		{"unified addition", false, 1, right, "20 + new"},
		{"unified left pane", false, 0, left, "10 - old"},
		{"split deletion", true, 0, left, "10 - old"},
		{"split addition", true, 0, right, "20 + new"},
		{"split unpaired addition", true, 1, right, "21 + extra"},
		{"split empty pane", true, 1, left, ""},
		{"unified context", false, 3, right, "11 22 shared"},
		{"split old context", true, 2, left, "11 shared"},
		{"split new context", true, 2, right, "22 shared"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := newDiffView(p, Options{SideBySide: tc.split})
			rendered := renderProjectedTestLine(v, tc.line, tc.pane)
			got := strings.Join(strings.Fields(ansi.Strip(rendered)), " ")
			testutil.RequireEqual(t, got, tc.want,
				"projected source line changed")
		})
	}
}

func TestRenderLineKeepsLayoutWhileScrolling(t *testing.T) {
	p := patchWithLines([]patch.Line{
		{Kind: patch.Deletion, Text: "abcdef", OldNumber: 10},
		{Kind: patch.Addition, Text: "abcdef", NewNumber: 20},
	})
	for _, tc := range []struct {
		name  string
		split bool
		pane  diffPane
		width int
		want  string
	}{
		{"unified", false, right, 22, "10 - cdef"},
		{"unified left identity", false, left, 22, "10 - cdef"},
		{"split old", true, left, 14, "10 abcdef"},
		{"split new", true, right, 14, "20 abcdef"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := newDiffView(p, Options{SideBySide: tc.split})
			cursor := mustFirst(t, v)
			cursor.pane = tc.pane
			renderer := diffRenderer{
				view: v, viewport: diffViewport{leftColumn: 2},
			}
			rendered := renderer.renderLine(cursor, tc.width)
			got := strings.Join(strings.Fields(ansi.Strip(rendered)), " ")
			testutil.RequireEqual(t, got, tc.want,
				"gutter or scrollable content changed")
			testutil.RequireEqual(t, lipgloss.Width(rendered), tc.width,
				"line width changed")
		})
	}
}

func renderProjectedTestLine(v *diffView, line int, pane diffPane) string {
	var rows []int
	for y, row := range v.rows {
		if row.kind == lineRow {
			rows = append(rows, y)
		}
	}
	y := rows[line]
	renderer := diffRenderer{
		view: v, cursor: diffCursor{row: y, pane: pane},
		viewport: diffViewport{width: 40},
	}
	if v.split {
		return renderer.renderLine(diffCursor{row: y, pane: pane}, 40)
	}
	return renderer.renderRow(y)
}

func TestRenderNonLineRowsUseFullWidth(t *testing.T) {
	const width = 81
	for _, tc := range []struct {
		kind rowKind
		text string
		want string
	}{
		{fileRow, "file.go", renderFileRow("file.go", width)},
		{metadataRow, "metadata", metadataStyle.Render("  metadata")},
		{hunkRow, "@@ hunk", hunkStyle.Render("@@ hunk")},
	} {
		for _, split := range []bool{false, true} {
			renderer := diffRenderer{
				view: &diffView{
					split: split,
					rows:  []entry{{kind: tc.kind, text: tc.text}},
				},
				viewport: diffViewport{width: width},
			}
			testutil.RequireEqual(t, renderer.renderRow(0), tc.want,
				"split=%t kind=%d: non-line row changed", split, tc.kind)
		}
	}
}

func TestSplitRendererDerivesPaneWidthsFromViewport(t *testing.T) {
	v := newDiffView(patchWithLines([]patch.Line{
		{Kind: patch.Context, Text: "shared", OldNumber: 1, NewNumber: 1},
	}), Options{SideBySide: true})
	y := slices.IndexFunc(v.rows, func(row entry) bool {
		return row.kind == lineRow
	})
	renderer := diffRenderer{view: v}
	for _, tc := range []struct {
		width, wantWidth int
	}{
		{20, 43},
		{43, 43},
		{80, 80},
		{81, 81},
	} {
		renderer.viewport.width = tc.width
		got := renderer.renderRow(y)
		testutil.RequireEqual(t, lipgloss.Width(got), tc.wantWidth,
			"viewport width=%d: unexpected row width", tc.width)
		testutil.RequireEqual(t, strings.Count(ansi.Strip(got), "shared"), 2,
			"viewport width=%d: both panes must contain code", tc.width)
	}
}

func TestRenderLineKeepsGutterWhileScrolling(t *testing.T) {
	const unifiedGutter = "    1     1   "
	for _, tc := range []struct {
		name   string
		split  bool
		offset int
		width  int
		want   string
	}{
		{"unscrolled", false, 0, 18, unifiedGutter + "abcd"},
		{"scrolled", false, 2, 18, unifiedGutter + "cdef"},
		{"past end", false, 8, 18, unifiedGutter + "    "},
		{"narrow", false, 2, 3, "   "},
		{"split unscrolled", true, 0, 12, "    1   abcd"},
		{"split scrolled", true, 2, 12, "    1 abcdef"},
		{"split past end", true, 20, 12, "    1       "},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := newDiffView(patchWithLines([]patch.Line{{
				Kind: patch.Context, Text: "abcdef",
				OldNumber: 1, NewNumber: 1,
			}}), Options{SideBySide: tc.split, Dark: true})
			renderer := diffRenderer{
				view:     v,
				viewport: diffViewport{leftColumn: tc.offset},
			}
			rendered := renderer.renderLine(mustFirst(t, v), tc.width)
			testutil.RequireEqual(t, ansi.Strip(rendered), tc.want,
				"code line layout changed")
		})
	}
}

func TestRenderStateIsLocalToEachCall(t *testing.T) {
	t.Run("unified", func(t *testing.T) {
		assertRenderStateIsLocal(t, newUnifiedView(longPatch(), true))
	})
	t.Run("split", func(t *testing.T) {
		assertRenderStateIsLocal(t, newSideBySideView(longPatch(), true))
	})
}

func assertRenderStateIsLocal(t *testing.T, v *diffView) {
	t.Helper()
	first := mustFirst(t, v)
	next, ok := v.move(first, forward)
	testutil.RequireEqual(t, ok, true, "second line missing")
	viewport := v.newViewport(80, 5)
	baseline := v.render(viewport, first, nil)
	selection := beginSelection(next)
	scrolled := v.scrollHorizontal(viewport, 3)
	for _, state := range []struct {
		name      string
		viewport  diffViewport
		cursor    diffCursor
		selection *diffSelection
	}{
		{"cursor", viewport, next, nil},
		{"selection", viewport, first, &selection},
		{"scroll", scrolled, first, nil},
		{"resize", v.resize(viewport, 70, 4), first, nil},
	} {
		t.Run(state.name, func(t *testing.T) {
			rendered := v.render(
				state.viewport, state.cursor, state.selection,
			)
			testutil.RequireEqual(t, slices.Equal(rendered, baseline), false,
				"render ignored changed %s", state.name)
			testutil.RequireEqual(t,
				slices.Equal(v.render(viewport, first, nil), baseline), true,
				"render retained changed %s", state.name)
		})
	}
}

func TestRenderReturnsPaddedViewportRows(t *testing.T) {
	for _, split := range []bool{false, true} {
		for _, height := range []int{1, 5, 20} {
			v := newDiffView(testPatch(), Options{
				SideBySide: split,
				Dark:       true,
			})
			viewport := v.newViewport(100, height)
			rows := v.render(viewport, mustFirst(t, v), nil)
			testutil.RequireEqual(t, len(rows), height,
				"split=%t: row count differs from viewport height", split)
			assertViewportRows(t, rows, len(v.rows))
		}
	}
}

func assertViewportRows(t *testing.T, rows []string, contentRows int) {
	t.Helper()
	for index, row := range rows {
		testutil.RequireEqual(t, strings.ContainsRune(row, '\n'), false,
			"row=%d: rendered row contains a newline", index)
		if index >= contentRows {
			testutil.RequireEqual(t, row, "",
				"row=%d: padding is not blank", index)
		}
	}
}
