package diffscreen

import (
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/eskelinenantti/review-my-slop/internal/ui"
)

func (v *diffView) newViewport(width, height int) diffViewport {
	return v.resize(diffViewport{}, width, height)
}

func (v *diffView) resize(
	viewport diffViewport,
	width, height int,
) diffViewport {
	viewport.width, viewport.height = max(1, width), max(1, height)
	return v.clampViewport(viewport)
}

func (v *diffView) maxTop(height int) int {
	end := max(0, len(v.rows)-height)
	return end + v.stickyHeaderHeight(end, height)
}

func (v *diffView) clampViewport(viewport diffViewport) diffViewport {
	viewport.top = max(0, min(viewport.top, v.maxTop(viewport.height)))
	viewport.leftColumn = max(
		0,
		min(viewport.leftColumn, v.maxHorizontalOffset(viewport.width)),
	)
	return viewport
}

// stickyHeaderHeight reserves a row only when the file header has scrolled out
// of view and there is room left for content.
func (v *diffView) stickyHeaderHeight(top int, viewportHeight int) int {
	if viewportHeight > 1 && top >= 0 &&
		top < len(v.rows) && v.rows[top].kind != fileRow {
		return 1
	}
	return 0
}

func (v *diffView) contentHeight(viewport diffViewport) int {
	headerHeight := v.stickyHeaderHeight(viewport.top, viewport.height)
	height := viewport.height - headerHeight
	return max(1, height)
}

// visibleRows returns the half-open range of projected rows in the viewport,
// excluding the space reserved for a sticky file header.
func (v *diffView) visibleRows(viewport diffViewport) (start, end int) {
	end = min(len(v.rows), viewport.top+v.contentHeight(viewport))
	return viewport.top, end
}

func (v *diffView) keepVisible(
	viewport diffViewport,
	cursor diffCursor,
) diffViewport {
	if !v.valid(cursor) {
		return v.clampViewport(viewport)
	}
	viewport = v.clampViewport(viewport)
	// The top row may toggle the sticky header, changing usable height.
	for range 2 {
		height := v.contentHeight(viewport)
		if cursor.row < viewport.top {
			viewport.top = cursor.row
		}
		if cursor.row >= viewport.top+height {
			viewport.top = cursor.row - height + 1
		}
		viewport = v.clampViewport(viewport)
	}
	return viewport
}

func (v *diffView) align(
	viewport diffViewport,
	cursor diffCursor,
	alignment verticalAlignment,
) diffViewport {
	headerHeight := 0
	if viewport.height > 1 {
		headerHeight = 1
	}
	offset := max(0, alignmentOffset(viewport.height, alignment)-headerHeight)
	viewport.top = cursor.row - offset
	if v.stickyHeaderHeight(viewport.top, viewport.height) == 0 {
		viewport.top = cursor.row - alignmentOffset(viewport.height, alignment)
	}
	return v.clampViewport(viewport)
}

func alignmentOffset(height int, alignment verticalAlignment) int {
	if alignment == middle {
		return height / 2
	}
	if alignment == bottom {
		return height - 1
	}
	return 0
}

func (v *diffView) scrollHorizontal(
	viewport diffViewport,
	columns int,
) diffViewport {
	viewport.leftColumn = ui.MoveWithinBounds(
		viewport.leftColumn, columns, v.maxHorizontalOffset(viewport.width),
	)
	return v.clampViewport(viewport)
}

func (v *diffView) scrollVertical(
	viewport diffViewport,
	cursor diffCursor,
	rows int,
) (diffViewport, diffCursor) {
	viewport.top = ui.MoveWithinBounds(
		viewport.top, rows, v.maxTop(viewport.height),
	)
	viewport = v.clampViewport(viewport)
	if !v.valid(cursor) || v.cursorVisible(viewport, cursor) {
		return viewport, cursor
	}
	start, end := v.visibleRows(viewport)
	target := max(start, min(cursor.row, end-1))
	d := forward
	if cursor.row > target {
		d = backward
	}
	candidate, ok := v.inPreferredPane(cursor.pane,
		func(pane diffPane) (diffCursor, bool) {
			return v.nearest(target, pane, d, viewport)
		})
	if ok {
		cursor = candidate
	}
	return viewport, cursor
}

func (v *diffView) cursorVisible(
	viewport diffViewport,
	cursor diffCursor,
) bool {
	return v.valid(cursor) && v.rowVisible(viewport, cursor.row)
}

func (v *diffView) scrollHalfPage(
	viewport diffViewport,
	cursor diffCursor,
	direction direction,
) (diffViewport, diffCursor) {
	if !v.valid(cursor) {
		return viewport, cursor
	}
	distance := int(direction) * max(1, viewport.height/2)
	viewport.top += distance
	viewport = v.clampViewport(viewport)
	target := min(len(v.rows)-1, max(0, cursor.row+distance))
	candidate, ok := v.nearest(target, cursor.pane, direction, viewport)
	if ok {
		cursor = candidate
	}
	return viewport, cursor
}

func (v *diffView) viewportProgress(viewport diffViewport) int {
	if len(v.rows) == 0 {
		return 0
	}
	_, end := v.visibleRows(viewport)
	return end * 100 / len(v.rows)
}

func (v *diffView) nearest(
	target int,
	pane diffPane,
	direction direction,
	viewport diffViewport,
) (diffCursor, bool) {
	return nearestCursor(target, v.contentHeight(viewport)-1, direction,
		func(row int) (diffCursor, bool) {
			return v.cursorInViewport(row, pane, viewport)
		})
}

func (v *diffView) cursorInViewport(
	y int,
	pane diffPane,
	viewport diffViewport,
) (diffCursor, bool) {
	if !v.rowVisible(viewport, y) {
		return diffCursor{}, false
	}
	return v.cursorAt(y, pane)
}

func (v *diffView) rowVisible(viewport diffViewport, row int) bool {
	start, end := v.visibleRows(viewport)
	return row >= start && row < end
}

func renderedLineWidth(current projectedRow) int {
	text := current.text + current.panes[left].text + current.panes[right].text
	return lipgloss.Width(
		expandTabs(ansi.Strip(text)),
	)
}
