package diffscreen

import (
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestSplitLayout(t *testing.T) {
	for _, tc := range []struct {
		name               string
		width, left, right int
	}{
		{"even panes", 121, 59, 59},
		{"uneven panes", 120, 58, 59},
		{"minimum panes", 10, 20, 20},
	} {
		t.Run(tc.name, func(t *testing.T) {
			layout := newSplitLayout(tc.width)
			testutil.RequireEqual(t, layout.leftWidth, tc.left,
				"left pane width")
			testutil.RequireEqual(t, layout.rightWidth, tc.right,
				"right pane width")
			rightStart := tc.left + ansi.StringWidth(splitDivider)
			for _, point := range []struct {
				x    int
				pane diffPane
				ok   bool
			}{
				{0, left, true},
				{tc.left - 1, left, true},
				{tc.left, right, false},
				{tc.left + 1, right, false},
				{rightStart - 1, right, false},
				{rightStart, right, true},
				{rightStart + tc.right - 1, right, true},
			} {
				pane, ok := layout.paneAt(point.x)
				testutil.RequireEqual(t, ok, point.ok,
					"selectable at %d", point.x)
				testutil.RequireEqual(t, pane, point.pane,
					"pane at %d", point.x)
			}
		})
	}
}
