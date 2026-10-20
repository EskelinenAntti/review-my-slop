package diffscreen

import (
	"charm.land/lipgloss/v2"

	"github.com/eskelinenantti/review-my-slop/internal/ui/internal/scroll"
)

func (v *diffView) scrollHorizontal(
	viewport diffViewport,
	columns int,
) diffViewport {
	viewport.leftColumn = scroll.Offset(
		viewport.leftColumn,
		columns,
		v.maxHorizontalOffset(viewport.width),
	)
	return v.clampViewport(viewport)
}

func (v *diffView) maxHorizontalOffset(width int) int {
	contentWidth := max(1, width-unifiedGutterWidth)
	extra := 0
	if v.split {
		contentWidth = max(1, splitPaneWidth(width)-splitGutterWidth)
		extra = splitMarkerWidth
	}
	return max(0, v.maxLineWidth+extra-contentWidth)
}

func renderedLineWidth(current entry) int {
	return lipgloss.Width(
		expandTabs(current.text + current.left + current.right),
	)
}
