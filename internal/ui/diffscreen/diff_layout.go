package diffscreen

const (
	lineNumberWidth    = 5
	unifiedGutterWidth = 2*lineNumberWidth + 4
	paneGutterWidth    = lineNumberWidth + 1
	paneMarkerWidth    = 2
	paneSeparator      = " │ "
	paneSeparatorWidth = 3
)

func splitPaneWidths(width int) (left, right int) {
	left = max(20, (width-paneSeparatorWidth)/2)
	return left, max(20, width-paneSeparatorWidth-left)
}

func (v *diffView) maxHorizontalOffset(width int) int {
	contentWidth := max(1, width-unifiedGutterWidth)
	extra := 0
	if v.split {
		contentWidth = max(1, (width-paneSeparatorWidth)/2-paneGutterWidth)
		extra = paneMarkerWidth
	}
	return max(0, v.maxLineWidth+extra-contentWidth)
}
