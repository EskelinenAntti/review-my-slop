package diffscreen

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

func (r diffRenderer) renderSplitRow(current entry, y int) string {
	leftWidth, rightWidth := splitPaneWidths(r.viewport.Width)
	left := r.renderPane(current, diffCursor{row: y, pane: left}, leftWidth)
	right := r.renderPane(current, diffCursor{row: y, pane: right}, rightWidth)
	return left + " │ " + right
}

func splitPaneWidths(width int) (left, right int) {
	left = max(20, (width-3)/2)
	return left, max(20, width-3-left)
}

func (r diffRenderer) renderPane(
	current entry,
	candidate diffCursor,
	width int,
) string {
	index := r.view.lineIndex(current, candidate.pane)
	if index < 0 {
		return strings.Repeat(" ", width)
	}
	line := r.view.patch.Files[current.file].Hunks[current.hunk].Lines[index]
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
	gutter := fmt.Sprintf("%5s ", number(numberValue))
	value := gutter + fitANSIWindow(
		prefix+text,
		r.viewport.LeftColumn,
		width-lipgloss.Width(gutter),
	)
	style := lineStyle(line.Kind, r.view.dark)
	colors := preserveForeground
	if selected(r.selection, candidate) {
		style, colors = selectionRowStyle(r.view.dark), discardForeground
	}
	if r.cursor == candidate {
		style, colors = cursorStyle, discardForeground
	}
	return renderStyledRow(style, value, width, colors)
}
