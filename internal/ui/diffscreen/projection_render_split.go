package diffscreen

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

func (r rowRenderer) renderSplitRow(current displayRow, y int) string {
	leftWidth, rightWidth := splitPaneWidths(r.viewport.Width)
	left := r.renderPane(current, patchCursor{row: y, pane: left}, leftWidth)
	right := r.renderPane(current, patchCursor{row: y, pane: right}, rightWidth)
	return left + " │ " + right
}

func splitPaneWidths(width int) (left, right int) {
	left = max(20, (width-3)/2)
	return left, max(20, width-3-left)
}

func (r rowRenderer) renderPane(
	current displayRow,
	candidate patchCursor,
	width int,
) string {
	index := r.projection.sourceLineIndex(current, candidate.pane)
	if index < 0 {
		return strings.Repeat(" ", width)
	}
	hunk := r.projection.patch.Files[current.file].Hunks[current.hunk]
	line := hunk.Lines[index]
	text := current.right
	numberValue := line.NewNumber
	if candidate.pane == left {
		text, numberValue = current.left, line.OldNumber
	}
	prefix := "  "
	if line.Kind == patch.Addition {
		prefix = addedStyle.Render("+") + " "
	}
	if line.Kind == patch.Deletion {
		prefix = removedStyle.Render("-") + " "
	}
	gutter := fmt.Sprintf("%5s ", formatLineNumber(numberValue))
	value := gutter + fitANSIWindow(
		prefix+text,
		r.viewport.LeftColumn,
		width-lipgloss.Width(gutter),
	)
	style := lineStyle(line.Kind, r.projection.dark)
	colors := preserveForeground
	if cursorIsSelected(r.selection, candidate) {
		style, colors = selectionRowStyle(r.projection.dark), discardForeground
	}
	if r.cursor == candidate {
		style, colors = cursorStyle, discardForeground
	}
	return renderStyledRow(style, value, width, colors)
}
