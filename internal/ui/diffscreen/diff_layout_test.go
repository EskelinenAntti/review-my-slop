package diffscreen

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestViewportHitTestingMatchesRenderedRows(t *testing.T) {
	for _, split := range []bool{false, true} {
		for _, test := range []struct {
			name string
			p    patch.Patch
			top  int
			text []string
			rows []int // -1 marks headers, metadata, and padding.
		}{
			{
				name: "natural header", p: stickyHeaderPatch(),
				text: []string{"first.go", "@@", "first one", "first two"},
				rows: []int{-1, -1, 2, 3},
			},
			{
				name: "sticky header", p: stickyHeaderPatch(), top: 3,
				text: []string{"first.go", "first two", "first three"},
				rows: []int{-1, 3, 4},
			},
			{
				name: "file transition", p: stickyHeaderPatch(), top: 4,
				text: []string{"first.go", "first three", "second.go", "@@"},
				rows: []int{-1, 4, -1, -1},
			},
			{
				name: "next file header", p: stickyHeaderPatch(), top: 5,
				text: []string{"second.go", "@@", "second one"},
				rows: []int{-1, -1, 7},
			},
			{
				name: "next file hunk", p: stickyHeaderPatch(), top: 6,
				text: []string{"second.go", "@@"},
				rows: []int{-1, -1},
			},
			{
				name: "last line", p: stickyHeaderPatch(), top: 7,
				text: []string{"second.go", "second one"},
				rows: []int{-1, 7},
			},
			{
				name: "one row", p: stickyHeaderPatch(), top: 3,
				text: []string{"first two"}, rows: []int{3},
			},
			{
				name: "padding", p: stickyHeaderPatch(),
				text: []string{
					"first.go", "@@", "first one", "first two", "first three",
					"second.go", "@@", "second one", "", "",
				},
				rows: []int{-1, -1, 2, 3, 4, -1, -1, 7, -1, -1},
			},
			{
				name: "metadata",
				p: patch.Patch{Files: []patch.File{{
					DisplayPath: "binary.dat",
					Metadata:    []string{"Binary files differ"},
				}}},
				top: 1, text: []string{"binary.dat", "Binary files differ"},
				rows: []int{-1, -1},
			},
			{
				name: "empty", text: []string{"", "", ""},
				rows: []int{-1, -1, -1},
			},
		} {
			name := fmt.Sprintf("%s/split=%v", test.name, split)
			t.Run(name, func(t *testing.T) {
				v := New(test.p, Options{SideBySide: split})
				v.Resize(121, len(test.text)+3)
				v.viewport.top = test.top
				checkViewportRows(t, v, test.text, test.rows)
			})
		}
	}
}

func checkViewportRows(t *testing.T, v *View, texts []string, rows []int) {
	t.Helper()
	rendered := v.view.renderRows(v.viewport, v.cursor, nil)
	testutil.RequireEqual(t, len(rendered), len(texts), "body height")
	for offset, text := range texts {
		row := ansi.Strip(rendered[offset])
		if !strings.Contains(row, text) || text == "" && row != "" {
			t.Errorf("body row %d = %q, want %q", offset, row, text)
		}
		for _, x := range []int{10, 100} {
			cursor, ok := v.cursorAtPoint(x, offset+1)
			want := rows[offset]
			testutil.RequireEqual(t, ok, want >= 0,
				"hit at (%d, %d)", x, offset+1)
			if ok {
				testutil.CheckEqual(t, cursor.row, want, "hit row")
				line, _ := v.view.line(cursor)
				testutil.CheckEqual(t, line.Text, text, "hit text")
			}
		}
	}
}

func TestSplitHitTestingMatchesRenderedPaneBoundaries(t *testing.T) {
	for _, width := range []int{100, 101, 120, 121} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			v := New(testPatch(), Options{SideBySide: true})
			v.Resize(width, 20)
			row := ansi.Strip(v.view.renderRows(v.viewport, v.cursor, nil)[2])
			divider := strings.Index(row, " │ ")
			if divider < 0 {
				t.Fatalf("rendered row has no divider: %q", row)
			}
			testutil.RequireEqual(t, v.BeginDrag(divider-1, 3), true,
				"last left-pane cell was ignored")
			testutil.RequireEqual(t, v.cursor.pane, left, "wrong left pane")
			for x := divider; x < divider+3; x++ {
				testutil.CheckEqual(t, v.BeginDrag(x, 3), false,
					"divider cell %d accepted", x)
			}
			testutil.RequireEqual(t, v.BeginDrag(divider+3, 3), true,
				"first right-pane cell was ignored")
			testutil.RequireEqual(t, v.cursor.pane, right, "wrong right pane")
		})
	}
}

func TestHorizontalScrollLimitsAccountForGutters(t *testing.T) {
	for _, tt := range []struct {
		name  string
		split bool
		width int
		want  int
	}{
		{"unified narrow", false, 14, 79},
		{"unified", false, 80, 14},
		{"unified fits", false, 100, 0},
		{"split even", true, 100, 40},
		{"split odd", true, 101, 39},
		{"split fits", true, 200, 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			v := &diffView{split: tt.split, maxLineWidth: 80}
			viewport := v.newViewport(tt.width, 10)
			viewport = v.scrollHorizontal(viewport, 1000)
			testutil.CheckEqual(t, viewport.leftColumn, tt.want,
				"horizontal scroll limit = %d, want %d",
				viewport.leftColumn, tt.want,
			)
			viewport = v.scrollHorizontal(viewport, -1000)
			testutil.CheckEqual(t, viewport.leftColumn, 0,
				"horizontal scroll did not return to origin")
		})
	}
}
