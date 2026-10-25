package diffscreen

import (
	"testing"

	"charm.land/lipgloss/v2"
)

func TestSplitPaneWidths(t *testing.T) {
	for _, tt := range []struct {
		width, left, right int
	}{
		{0, 20, 20},
		{40, 20, 20},
		{43, 20, 20},
		{80, 38, 39},
		{81, 39, 39},
	} {
		left, right := splitPaneWidths(tt.width)
		if left != tt.left || right != tt.right {
			t.Errorf("splitPaneWidths(%d) = (%d, %d), want (%d, %d)",
				tt.width, left, right, tt.left, tt.right)
		}
	}
	if width := lipgloss.Width(paneSeparator); width != paneSeparatorWidth {
		t.Errorf("separator width = %d, want %d", width, paneSeparatorWidth)
	}
}

func TestMaxHorizontalOffset(t *testing.T) {
	for _, tt := range []struct {
		name                   string
		split                  bool
		width, lineWidth, want int
	}{
		{"unified", false, 80, 100, 34},
		{"split even", true, 80, 100, 70},
		{"split odd", true, 81, 100, 69},
		{"unified narrow", false, 1, 100, 99},
		{"split narrow", true, 1, 100, 101},
		{"unified fits", false, 80, 10, 0},
		{"split fits", true, 80, 10, 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			v := diffView{split: tt.split, maxLineWidth: tt.lineWidth}
			if got := v.maxHorizontalOffset(tt.width); got != tt.want {
				t.Errorf("maxHorizontalOffset(%d) = %d, want %d",
					tt.width, got, tt.want)
			}
		})
	}
}
