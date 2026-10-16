package diffscreen

import (
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func (v *diffView) newVisibleArea(width, height int) diffViewport {
	return v.resize(diffViewport{}, width, height)
}

func (v *diffView) resize(
	viewport diffViewport,
	width, height int,
) diffViewport {
	viewport.Width, viewport.Height = max(1, width), max(1, height)
	return v.keepVisibleAreaOnScreen(viewport)
}

func (v *diffView) maximumVisibleAreaTop(height int) int {
	end := max(0, len(v.rows)-height)
	if v.fileHeaderStaysAtTop(end, height) {
		end++
	}
	return end
}

func (v *diffView) keepVisibleAreaOnScreen(viewport diffViewport) diffViewport {
	viewport.top = max(
		0,
		min(viewport.top, v.maximumVisibleAreaTop(viewport.Height)),
	)
	viewport.LeftColumn = max(
		0,
		min(viewport.LeftColumn, v.maximumVisibleColumnOffset(viewport.Width)),
	)
	return viewport
}

func (v *diffView) fileHeaderStaysAtTop(top int, viewportHeight int) bool {
	return viewportHeight > 1 && top >= 0 &&
		top < len(v.rows) && v.rows[top].kind != fileRow
}

func (v *diffView) visibleRowsBelowHeader(viewport diffViewport) int {
	height := viewport.Height
	if v.fileHeaderStaysAtTop(viewport.top, viewport.Height) {
		height--
	}
	return max(1, height)
}

func (v *diffView) keepFocusedLineVisible(
	viewport diffViewport,
	cursor diffCursor,
) diffViewport {
	if !v.isChangedLine(cursor) {
		return v.keepVisibleAreaOnScreen(viewport)
	}
	viewport = v.keepVisibleAreaOnScreen(viewport)
	for range 2 {
		height := v.visibleRowsBelowHeader(viewport)
		if cursor.row < viewport.top {
			viewport.top = cursor.row
		}
		if cursor.row >= viewport.top+height {
			viewport.top = cursor.row - height + 1
		}
		viewport = v.keepVisibleAreaOnScreen(viewport)
	}
	return viewport
}

func (v *diffView) positionFocusedLine(
	viewport diffViewport,
	cursor diffCursor,
	alignment verticalAlignment,
) diffViewport {
	headerHeight := 0
	if viewport.Height > 1 {
		headerHeight = 1
	}
	offset := max(
		0,
		rowsBeforeAlignedLine(viewport.Height, alignment)-headerHeight,
	)
	viewport.top = cursor.row - offset
	if !v.fileHeaderStaysAtTop(viewport.top, viewport.Height) {
		viewport.top = cursor.row - rowsBeforeAlignedLine(
			viewport.Height,
			alignment,
		)
	}
	return v.keepVisibleAreaOnScreen(viewport)
}

func rowsBeforeAlignedLine(height int, alignment verticalAlignment) int {
	if alignment == middle {
		return height / 2
	}
	if alignment == bottom {
		return height - 1
	}
	return 0
}

func (v *diffView) scrollVisibleAreaSideways(
	viewport diffViewport,
	columns int,
) diffViewport {
	maximumColumn := v.maximumVisibleColumnOffset(viewport.Width)
	maximumScroll := maximumColumn - viewport.LeftColumn
	columns = max(
		-viewport.LeftColumn,
		min(columns, maximumScroll),
	)
	viewport.LeftColumn += columns
	return v.keepVisibleAreaOnScreen(viewport)
}

func (v *diffView) scrollVisibleAreaUpDown(
	viewport diffViewport,
	cursor diffCursor,
	rows int,
) (diffViewport, diffCursor) {
	rows = max(
		-viewport.top,
		min(rows, v.maximumVisibleAreaTop(viewport.Height)-viewport.top),
	)
	viewport.top += rows
	viewport = v.keepVisibleAreaOnScreen(viewport)
	if !v.isChangedLine(cursor) || v.isFocusedLineVisible(viewport, cursor) {
		return viewport, cursor
	}
	target := max(
		viewport.top,
		min(cursor.row, viewport.top+v.visibleRowsBelowHeader(viewport)-1),
	)
	d := forward
	if cursor.row > target {
		d = backward
	}
	candidate, ok := v.findVisibleLineNear(target, cursor.pane, d, viewport)
	if !ok && v.split {
		candidate, ok = v.findVisibleLineNear(
			target,
			cursor.pane.oppositePane(),
			d,
			viewport,
		)
	}
	if ok {
		cursor = candidate
	}
	return viewport, cursor
}

func (v *diffView) isFocusedLineVisible(
	viewport diffViewport,
	cursor diffCursor,
) bool {
	return v.isChangedLine(cursor) && cursor.row >= viewport.top &&
		cursor.row < viewport.top+v.visibleRowsBelowHeader(viewport)
}

func (v *diffView) moveFocusedLineHalfPage(
	viewport diffViewport,
	cursor diffCursor,
	direction direction,
) (diffViewport, diffCursor) {
	if !v.isChangedLine(cursor) {
		return viewport, cursor
	}
	distance := int(direction) * max(1, viewport.Height/2)
	viewport.top += distance
	viewport = v.keepVisibleAreaOnScreen(viewport)
	target := min(len(v.rows)-1, max(0, cursor.row+distance))
	candidate, ok := v.findVisibleLineNear(
		target,
		cursor.pane,
		direction,
		viewport,
	)
	if ok {
		cursor = candidate
	}
	return viewport, cursor
}

func (v *diffView) visibleContentProgressPercent(viewport diffViewport) int {
	if len(v.rows) == 0 {
		return 0
	}
	bottom := min(len(v.rows), viewport.top+v.visibleRowsBelowHeader(viewport))
	return bottom * 100 / len(v.rows)
}

func (v *diffView) findVisibleLineNear(
	target int,
	pane diffPane,
	direction direction,
	viewport diffViewport,
) (diffCursor, bool) {
	height := v.visibleRowsBelowHeader(viewport)
	for distance := 0; distance < height; distance++ {
		for _, y := range []int{
			target + int(direction)*distance,
			target - int(direction)*distance,
		} {
			if cursor, ok := v.isLineVisible(y, pane, viewport); ok {
				return cursor, true
			}
		}
	}
	return diffCursor{}, false
}

func (v *diffView) isLineVisible(
	y int,
	pane diffPane,
	viewport diffViewport,
) (diffCursor, bool) {
	if y < viewport.top ||
		y >= viewport.top+v.visibleRowsBelowHeader(viewport) {
		return diffCursor{}, false
	}
	return v.changedLineAtRow(y, pane)
}

func (v *diffView) maximumVisibleColumnOffset(width int) int {
	contentWidth := max(1, width-14)
	extra := 0
	if v.split {
		contentWidth, extra = max(1, (width-3)/2-6), 2
	}
	longest := 0
	for _, current := range v.rows {
		if current.kind != lineRow {
			continue
		}
		longest = max(
			longest,
			lipgloss.Width(
				replaceTabsWithSpaces(
					ansi.Strip(current.text+current.left+current.right),
				),
			)+extra,
		)
	}
	return max(0, longest-contentWidth)
}
