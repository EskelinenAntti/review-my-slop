package diffscreen

import (
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

func TestLineDecorations(t *testing.T) {
	for _, tt := range []struct {
		name    string
		line    patch.Line
		unified string
		old     string
		new     string
		marker  string
	}{
		{"context", patch.Line{OldNumber: 12, NewNumber: 34},
			"   12    34   ", "   12 ", "   34 ", "  "},
		{"addition", patch.Line{Kind: patch.Addition, NewNumber: 34},
			"         34 + ", "      ", "   34 ", "+ "},
		{"deletion", patch.Line{Kind: patch.Deletion, OldNumber: 12},
			"   12       - ", "   12 ", "      ", "- "},
		{"large numbers", patch.Line{OldNumber: 123456, NewNumber: 234567},
			"123456 234567   ", "123456 ", "234567 ", "  "},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := ansi.Strip(unifiedGutter(tt.line)); got != tt.unified {
				t.Errorf("unified gutter = %q, want %q", got, tt.unified)
			}
			if got := splitGutter(tt.line.OldNumber); got != tt.old {
				t.Errorf("old gutter = %q, want %q", got, tt.old)
			}
			if got := splitGutter(tt.line.NewNumber); got != tt.new {
				t.Errorf("new gutter = %q, want %q", got, tt.new)
			}
			if got := ansi.Strip(splitMarker(tt.line.Kind)); got != tt.marker {
				t.Errorf("split marker = %q, want %q", got, tt.marker)
			}
		})
	}
}

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
