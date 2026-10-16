package diffscreen

import (
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func (v *diffView) newViewport(width, height int) diffViewport {
	return v.resizeViewport(diffViewport{}, width, height)
}

func (v *diffView) resizeViewport(
	viewport diffViewport,
	width, height int,
) diffViewport {
	viewport.Width, viewport.Height = max(1, width), max(1, height)
	return v.clampViewport(viewport)
}

func (v *diffView) maxScrollTop(viewportHeight int) int {
	end := max(0, len(v.rows)-viewportHeight)
	if v.shouldShowStickyHeader(end, viewportHeight) {
		end++
	}
	return end
}

func (v *diffView) clampViewport(viewport diffViewport) diffViewport {
	viewport.top = max(0, min(viewport.top, v.maxScrollTop(viewport.Height)))
	viewport.LeftColumn = max(
		0,
		min(viewport.LeftColumn, v.maxHorizontalScrollOffset(viewport.Width)),
	)
	return viewport
}

func (v *diffView) shouldShowStickyHeader(topRow int, viewportHeight int) bool {
	return viewportHeight > 1 && topRow >= 0 &&
		topRow < len(v.rows) && v.rows[topRow].kind != fileRow
}

func (v *diffView) viewportBodyHeight(viewport diffViewport) int {
	height := viewport.Height
	if v.shouldShowStickyHeader(viewport.top, viewport.Height) {
		height--
	}
	return max(1, height)
}

func (v *diffView) keepCursorVisible(
	viewport diffViewport,
	cursor diffCursor,
) diffViewport {
	if !v.valid(cursor) {
		return v.clampViewport(viewport)
	}
	viewport = v.clampViewport(viewport)
	for range 2 {
		height := v.viewportBodyHeight(viewport)
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
	if viewport.Height > 1 {
		headerHeight = 1
	}
	offset := max(0, verticalOffsetForAlignment(viewport.Height, alignment)-headerHeight)
	viewport.top = cursor.row - offset
	if !v.shouldShowStickyHeader(viewport.top, viewport.Height) {
		viewport.top = cursor.row - verticalOffsetForAlignment(viewport.Height, alignment)
	}
	return v.clampViewport(viewport)
}

func verticalOffsetForAlignment(height int, alignment verticalAlignment) int {
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
	columns = max(
		-viewport.LeftColumn,
		min(columns, v.maxHorizontalScrollOffset(viewport.Width)-viewport.LeftColumn),
	)
	viewport.LeftColumn += columns
	return v.clampViewport(viewport)
}

func (v *diffView) scrollVertical(
	viewport diffViewport,
	cursor diffCursor,
	rowDelta int,
) (updatedViewport diffViewport, visibleCursor diffCursor) {
	rowDelta = max(
		-viewport.top,
		min(rowDelta, v.maxScrollTop(viewport.Height)-viewport.top),
	)
	viewport.top += rowDelta
	viewport = v.clampViewport(viewport)
	if !v.valid(cursor) || v.isCursorVisible(viewport, cursor) {
		return viewport, cursor
	}
	target := max(
		viewport.top,
		min(cursor.row, viewport.top+v.viewportBodyHeight(viewport)-1),
	)
	d := forward
	if cursor.row > target {
		d = backward
	}
	candidate, ok := v.nearestCursorInVisibleRows(target, cursor.pane, d, viewport)
	if !ok && v.split {
		candidate, ok = v.nearestCursorInVisibleRows(target, cursor.pane.otherPane(), d, viewport)
	}
	if ok {
		cursor = candidate
	}
	return viewport, cursor
}

func (v *diffView) isCursorVisible(
	viewport diffViewport,
	cursor diffCursor,
) bool {
	return v.valid(cursor) && cursor.row >= viewport.top &&
		cursor.row < viewport.top+v.viewportBodyHeight(viewport)
}

func (v *diffView) scrollHalfPage(
	viewport diffViewport,
	cursor diffCursor,
	direction direction,
) (updatedViewport diffViewport, visibleCursor diffCursor) {
	if !v.valid(cursor) {
		return viewport, cursor
	}
	distance := int(direction) * max(1, viewport.Height/2)
	viewport.top += distance
	viewport = v.clampViewport(viewport)
	target := min(len(v.rows)-1, max(0, cursor.row+distance))
	candidate, ok := v.nearestCursorInVisibleRows(target, cursor.pane, direction, viewport)
	if ok {
		cursor = candidate
	}
	return viewport, cursor
}

func (v *diffView) scrollProgressPercent(viewport diffViewport) (percent int) {
	if len(v.rows) == 0 {
		return 0
	}
	bottom := min(len(v.rows), viewport.top+v.viewportBodyHeight(viewport))
	return bottom * 100 / len(v.rows)
}

func (v *diffView) nearestCursorInVisibleRows(
	targetRow int,
	pane diffPane,
	direction direction,
	viewport diffViewport,
) (cursor diffCursor, found bool) {
	height := v.viewportBodyHeight(viewport)
	for distance := 0; distance < height; distance++ {
		for _, y := range []int{
			targetRow + int(direction)*distance,
			targetRow - int(direction)*distance,
		} {
			if cursor, ok := v.cursorInViewport(y, pane, viewport); ok {
				return cursor, true
			}
		}
	}
	return diffCursor{}, false
}

func (v *diffView) cursorInViewport(
	row int,
	pane diffPane,
	viewport diffViewport,
) (diffCursor, bool) {
	if row < viewport.top || row >= viewport.top+v.viewportBodyHeight(viewport) {
		return diffCursor{}, false
	}
	return v.cursorAt(row, pane)
}

func (v *diffView) maxHorizontalScrollOffset(width int) int {
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
				expandTabs(ansi.Strip(current.text+current.left+current.right)),
			)+extra,
		)
	}
	return max(0, longest-contentWidth)
}
