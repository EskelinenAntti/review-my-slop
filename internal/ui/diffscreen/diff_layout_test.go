package diffscreen

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

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
