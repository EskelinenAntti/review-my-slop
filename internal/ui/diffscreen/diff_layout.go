package diffscreen

import "github.com/charmbracelet/x/ansi"

const splitDivider = " │ "

// Gutter widths include separators; split rows scroll the marker with the text.
const (
	lineNumberWidth    = 5
	unifiedGutterWidth = 2*lineNumberWidth + 4
	splitGutterWidth   = lineNumberWidth + 1
	splitMarkerWidth   = 2
)

// splitLayout shares pane boundaries between rendering and hit testing.
type splitLayout struct {
	leftWidth, rightWidth int
}

func newSplitLayout(width int) splitLayout {
	left := max(20, splitPaneWidth(width))
	return splitLayout{
		leftWidth:  left,
		rightWidth: max(20, width-ansi.StringWidth(splitDivider)-left),
	}
}

// splitPaneWidth excludes the divider, before minimum rendering widths apply.
func splitPaneWidth(width int) int {
	return (width - ansi.StringWidth(splitDivider)) / 2
}

func (l splitLayout) paneAt(x int) (diffPane, bool) {
	switch {
	case x < l.leftWidth:
		return left, true
	case x < l.leftWidth+ansi.StringWidth(splitDivider):
		return right, false
	default:
		return right, true
	}
}
