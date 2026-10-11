package diffscreen

import (
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func (v *diffView) newViewport(width, height int) diffViewport {
	return v.resize(diffViewport{}, width, height)
}

func (v *diffView) resize(
	viewport diffViewport,
	width,
	height int,
) diffViewport {
	viewport.Width, viewport.Height = max(one, width), max(one, height)
	return v.clampViewport(viewport)
}

func (v *diffView) maxTop(height int) int {
	end := max(zero, len(v.rows)-height)
	if v.hasStickyHeader(end, height) {
		end++
	}
	return end
}

func (v *diffView) clampViewport(viewport diffViewport) diffViewport {
	viewport.top = max(zero, min(viewport.top, v.maxTop(viewport.Height)))
	viewport.LeftColumn = max(
		zero,
		min(viewport.LeftColumn, v.maxHorizontalOffset(viewport.Width)),
	)
	return viewport
}

func (v *diffView) hasStickyHeader(top int, viewportHeight int) bool {
	return viewportHeight > one && top >= zero && top < len(v.rows) &&
		v.rows[top].kind != fileRow
}

func (v *diffView) contentHeight(viewport diffViewport) int {
	height := viewport.Height
	if v.hasStickyHeader(viewport.top, viewport.Height) {
		height--
	}
	return max(one, height)
}

func (v *diffView) keepVisible(
	initialViewport diffViewport,
	cursor diffCursor,
) diffViewport {
	viewport := initialViewport

	if !v.valid(cursor) {
		return v.clampViewport(viewport)
	}
	viewport = v.clampViewport(viewport)
	for range two {
		height := v.contentHeight(viewport)
		if cursor.row < viewport.top {
			viewport.top = cursor.row
		}
		if cursor.row >= viewport.top+height {
			viewport.top = cursor.row - height + one
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
	headerHeight := zero
	if viewport.Height > one {
		headerHeight = one
	}
	offset := max(
		zero,
		alignmentOffset(viewport.Height, alignment)-headerHeight,
	)
	viewport.top = cursor.row - offset
	if !v.hasStickyHeader(viewport.top, viewport.Height) {
		viewport.top = cursor.row - alignmentOffset(
			viewport.Height,
			alignment,
		)
	}
	return v.clampViewport(viewport)
}

func alignmentOffset(height int, alignment verticalAlignment) int {
	if alignment == middle {
		return height / two
	}
	if alignment == bottom {
		return height - one
	}
	return zero
}

func (v *diffView) scrollHorizontal(
	viewport diffViewport,
	initialColumns int,
) diffViewport {
	columns := initialColumns

	columns = max(
		-viewport.LeftColumn,
		min(
			columns,
			v.maxHorizontalOffset(viewport.Width)-viewport.LeftColumn,
		),
	)
	viewport.LeftColumn += columns
	return v.clampViewport(viewport)
}

func (v *diffView) scrollVertical(
	initialViewport diffViewport,
	initialCursor diffCursor,
	initialRows int,
) (
	diffViewport,
	diffCursor,
) {
	cursor := initialCursor

	viewport := initialViewport

	rows := initialRows

	rows = max(
		-viewport.top,
		min(rows, v.maxTop(viewport.Height)-viewport.top),
	)
	viewport.top += rows
	viewport = v.clampViewport(viewport)
	if !v.valid(cursor) || v.cursorVisible(viewport, cursor) {
		return viewport, cursor
	}
	target := max(
		viewport.top,
		min(cursor.row, viewport.top+v.contentHeight(viewport)-one),
	)
	d := forward
	if cursor.row > target {
		d = backward
	}
	candidate, ok := v.nearest(target, cursor.pane, d, viewport)
	if !ok && v.split {
		candidate, ok = v.nearest(target, cursor.pane.other(), d, viewport)
	}
	if ok {
		cursor = candidate
	}
	return viewport, cursor
}

func (v *diffView) cursorVisible(
	viewport diffViewport,
	cursor diffCursor,
) bool {
	return cursor.row >= viewport.top && v.valid(cursor) &&
		cursor.row < viewport.top+v.contentHeight(viewport)
}

func (v *diffView) scrollHalfPage(
	initialViewport diffViewport,
	initialCursor diffCursor,
	direction direction,
) (
	diffViewport,
	diffCursor,
) {
	cursor := initialCursor

	viewport := initialViewport

	if !v.valid(cursor) {
		return viewport, cursor
	}
	distance := int(direction) * max(one, viewport.Height/two)
	viewport.top += distance
	viewport = v.clampViewport(viewport)
	target := min(len(v.rows)-one, max(zero, cursor.row+distance))
	candidate, ok := v.nearest(target, cursor.pane, direction, viewport)
	if ok {
		cursor = candidate
	}
	return viewport, cursor
}

func (v *diffView) viewportProgress(viewport diffViewport) int {
	if len(v.rows) == zero {
		return zero
	}
	bottom := min(len(v.rows), viewport.top+v.contentHeight(viewport))
	return bottom * hundred / len(v.rows)
}

func (v *diffView) nearest(
	target int,
	pane diffPane,
	direction direction,
	viewport diffViewport,
) (
	diffCursor,
	bool,
) {
	height := v.contentHeight(viewport)
	for distance := zero; distance < height; distance++ {
		cursor, ok := v.viewportCursorAtDistance(
			target,
			pane,
			direction,
			viewport,
			height,
			distance,
		)
		if ok {
			return cursor, true
		}
	}
	return diffCursor{}, false
}

func (v *diffView) rowInViewport(
	y int,
	viewport diffViewport,
	height int,
) bool {
	return y >= viewport.top && y < viewport.top+height && y < len(v.rows)
}

func (v *diffView) maxHorizontalOffset(width int) int {
	contentWidth := max(one, width-14)
	extra := zero
	if v.split {
		contentWidth, extra = max(one, (width-3)/two-6), two
	}
	longest := zero
	for _, current := range v.rows {
		if current.kind != lineRow {
			continue
		}
		longest = max(
			longest,
			lipgloss.Width(
				expandTabs(
					ansi.Strip(current.text+current.left+current.right),
				),
			)+
				extra,
		)
	}
	return max(zero, longest-contentWidth)
}

func (v *diffView) viewportCursorAtDistance(
	target int,
	pane diffPane,
	direction direction,
	viewport diffViewport,
	height, distance int,
) (
	diffCursor,
	bool,
) {
	for _, y := range []int{
		target + int(direction)*distance,
		target - int(direction)*distance,
	} {
		if !v.rowInViewport(y, viewport, height) {
			continue
		}
		if cursor, ok := v.cursorAt(y, pane); ok {
			return cursor, true
		}
	}
	return diffCursor{}, false
}
