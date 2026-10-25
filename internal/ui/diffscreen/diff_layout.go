package diffscreen

const (
	lineNumberWidth    = 5
	unifiedGutterWidth = 2*lineNumberWidth + 4
	splitGutterWidth   = lineNumberWidth + 1
	splitMarkerWidth   = 2
	paneSeparator      = " │ "
	paneSeparatorWidth = 3
	minimumPaneWidth   = 20
)

func splitPaneWidths(width int) (left, right int) {
	left = max(minimumPaneWidth, (width-paneSeparatorWidth)/2)
	return left, max(minimumPaneWidth, width-paneSeparatorWidth-left)
}

func (v *diffView) maxHorizontalOffset(width int) int {
	contentWidth := max(1, width-unifiedGutterWidth)
	extra := 0
	if v.split {
		// Scrolling uses the nominal smaller pane, even when rendering clamps
		// narrow panes to minimumPaneWidth. The diff marker scrolls with text.
		contentWidth = max(1, (width-paneSeparatorWidth)/2-splitGutterWidth)
		extra = splitMarkerWidth
	}
	return max(0, v.maxLineWidth+extra-contentWidth)
}
