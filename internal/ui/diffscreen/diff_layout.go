package diffscreen

import (
	"fmt"
	"strconv"

	"charm.land/lipgloss/v2"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

const (
	unifiedGutterFormat = "%5s %5s %s "
	splitGutterFormat   = "%5s "
	paneSeparator       = " │ "
	minimumPaneWidth    = 20
)

// Scrolling and hit-testing use the same decorations as rendering. Measure a
// blank line to retain the nominal gutter width even for larger line numbers.
var (
	unifiedGutterWidth = lipgloss.Width(unifiedGutter(patch.Line{}))
	splitGutterWidth   = lipgloss.Width(splitGutter(0))
	splitMarkerWidth   = lipgloss.Width(splitMarker(patch.Context))
	paneSeparatorWidth = lipgloss.Width(paneSeparator)
)

func unifiedGutter(line patch.Line) string {
	return fmt.Sprintf(unifiedGutterFormat,
		number(line.OldNumber), number(line.NewNumber), lineMarker(line.Kind))
}

func splitGutter(value patch.LineNumber) string {
	return fmt.Sprintf(splitGutterFormat, number(value))
}

func splitMarker(kind patch.LineKind) string {
	return lineMarker(kind) + " "
}

func number(value patch.LineNumber) string {
	if value == 0 {
		return ""
	}
	return strconv.Itoa(int(value))
}

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
