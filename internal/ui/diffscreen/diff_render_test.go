package diffscreen

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestRenderProjectedLineNumbers(t *testing.T) {
	p := patchWithLines([]patch.Line{
		{Kind: patch.Deletion, Text: "old", OldNumber: 10},
		{Kind: patch.Addition, Text: "new", NewNumber: 20},
		{Kind: patch.Addition, Text: "extra", NewNumber: 21},
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
		return renderer.renderPane(v.rows[y], y, pane, 40)
	}
	return renderer.renderUnifiedRow(v.rows[y], y)
}

func TestRenderCodeLineKeepsGutterWhileScrolling(t *testing.T) {
	for _, tc := range []struct {
		name   string
		gutter string
		offset int
		width  int
		want   string
	}{
		{"unscrolled", "  1 ", 0, 8, "  1 abcd"},
		{"scrolled", "  1 ", 2, 8, "  1 cdef"},
		{"past end", "  1 ", 8, 8, "  1     "},
		{"narrow", "  1 ", 2, 3, "  1"},
		{"styled gutter", "\x1b[31m+\x1b[0m ", 2, 6, "+ cdef"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			renderer := diffRenderer{
				view:     newUnifiedView(longPatch(), true),
				viewport: diffViewport{leftColumn: tc.offset},
			}
			rendered := renderer.renderCodeLine(
				tc.gutter, "\x1b[32mabcdef\x1b[0m", tc.width,
				patch.Line{Kind: patch.Context}, diffCursor{},
			)
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
			testutil.RequireEqual(t, rendered != baseline, true,
				"render ignored changed %s", state.name)
			testutil.RequireEqual(t,
				v.render(viewport, first, nil), baseline,
				"render retained changed %s", state.name)
		})
	}
}
