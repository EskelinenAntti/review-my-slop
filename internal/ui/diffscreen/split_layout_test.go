package diffscreen

import (
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestSplitLayoutWidths(t *testing.T) {
	for _, tt := range []struct {
		name               string
		width, left, right int
	}{
		{name: "even", width: 120, left: 58, right: 59},
		{name: "odd", width: 121, left: 59, right: 59},
		{name: "minimum", width: 43, left: 20, right: 20},
		{name: "narrow", width: 10, left: 20, right: 20},
	} {
		t.Run(tt.name, func(t *testing.T) {
			layout := newSplitLayout(tt.width)
			testutil.RequireEqual(t, layout.leftWidth, tt.left,
				"left width = %d, want %d", layout.leftWidth, tt.left)
			testutil.RequireEqual(t, layout.rightWidth, tt.right,
				"right width = %d, want %d", layout.rightWidth, tt.right)
		})
	}
}

func TestSplitLayoutPaneBoundaries(t *testing.T) {
	for _, tt := range []struct {
		name string
		x    int
		pane diffPane
		ok   bool
	}{
		{name: "before layout", x: -1},
		{name: "left start", x: 0, pane: left, ok: true},
		{name: "left end", x: 57, pane: left, ok: true},
		{name: "divider start", x: 58},
		{name: "divider middle", x: 59},
		{name: "divider end", x: 60},
		{name: "right start", x: 61, pane: right, ok: true},
		{name: "right end", x: 119, pane: right, ok: true},
		{name: "after layout", x: 120},
	} {
		t.Run(tt.name, func(t *testing.T) {
			pane, ok := newSplitLayout(120).paneAt(tt.x)
			testutil.RequireEqual(t, ok, tt.ok,
				"paneAt(%d) valid = %v, want %v", tt.x, ok, tt.ok)
			if ok {
				testutil.RequireEqual(t, pane, tt.pane,
					"paneAt(%d) = %v, want %v", tt.x, pane, tt.pane)
			}
		})
	}
}
