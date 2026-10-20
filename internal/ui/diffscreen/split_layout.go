package diffscreen

import "charm.land/lipgloss/v2"

const splitPaneDivider = " │ "

type splitLayout struct {
	leftWidth, rightWidth int
}

func newSplitLayout(width int) splitLayout {
	available := width - lipgloss.Width(splitPaneDivider)
	leftWidth := max(20, available/2)
	return splitLayout{
		leftWidth:  leftWidth,
		rightWidth: max(20, available-leftWidth),
	}
}

func (l splitLayout) paneAt(x int) (diffPane, bool) {
	rightStart := l.leftWidth + lipgloss.Width(splitPaneDivider)
	switch {
	case x < 0:
		return right, false
	case x < l.leftWidth:
		return left, true
	case x < rightStart:
		return right, false
	case x < rightStart+l.rightWidth:
		return right, true
	default:
		return right, false
	}
}
