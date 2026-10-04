package diffscreen

import (
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func (v *diffView) newViewport(width, height int) diffViewport {
	return v.resize(diffViewport{}, width, height)
}

func (v *diffView) resize(viewport diffViewport, width, height int) diffViewport {
	viewport.Width, viewport.Height = max(1, width), max(1, height)
	return v.clampViewport(viewport)
}

func (v *diffView) clampViewport(viewport diffViewport) diffViewport {
	maxTop := max(0, len(v.rows)-viewport.Height)
	if v.hasStickyHeader(maxTop, viewport.Height) {
		maxTop++
	}
	viewport.top = max(0, min(viewport.top, maxTop))
	viewport.LeftColumn = max(0, min(viewport.LeftColumn, v.maxHorizontalOffset(viewport.Width)))
	return viewport
}

func (v *diffView) hasStickyHeader(top int, viewportHeight int) bool {
	return viewportHeight > 1 && top >= 0 && top < len(v.rows) && v.rows[top].kind != fileRow
}

func (v *diffView) contentHeight(viewport diffViewport) int {
	height := viewport.Height
	if v.hasStickyHeader(viewport.top, viewport.Height) {
		height--
	}
	return max(1, height)
}

func (v *diffView) keepVisible(viewport diffViewport, cursor diffCursor) diffViewport {
	if !v.valid(cursor) {
		return v.clampViewport(viewport)
	}
	viewport = v.clampViewport(viewport)
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

func (v *diffView) align(viewport diffViewport, cursor diffCursor, alignment verticalAlignment) diffViewport {
	headerHeight := 0
	if viewport.Height > 1 {
		headerHeight = 1
	}
	offset := max(0, alignmentOffset(viewport.Height, alignment)-headerHeight)
	viewport.top = cursor.row - offset
	if !v.hasStickyHeader(viewport.top, viewport.Height) {
		viewport.top = cursor.row - alignmentOffset(viewport.Height, alignment)
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

func (v *diffView) scrollHorizontal(viewport diffViewport, columns int) diffViewport {
	columns = max(-viewport.LeftColumn, min(columns, v.maxHorizontalOffset(viewport.Width)-viewport.LeftColumn))
	viewport.LeftColumn += columns
	return v.clampViewport(viewport)
}

func (v *diffView) scrollHalfPage(viewport diffViewport, cursor diffCursor, direction direction) (diffViewport, diffCursor) {
	if !v.valid(cursor) {
		return viewport, cursor
	}
	distance := int(direction) * max(1, viewport.Height/2)
	viewport.top += distance
	viewport = v.clampViewport(viewport)
	target := min(len(v.rows)-1, max(0, cursor.row+distance))
	if candidate, ok := v.nearest(target, cursor.pane, direction, viewport); ok {
		cursor = candidate
	}
	return viewport, cursor
}

func (v *diffView) viewportProgress(viewport diffViewport) int {
	if len(v.rows) == 0 {
		return 0
	}
	bottom := min(len(v.rows), viewport.top+v.contentHeight(viewport))
	return bottom * 100 / len(v.rows)
}

func (v *diffView) nearest(target int, pane diffPane, direction direction, viewport diffViewport) (diffCursor, bool) {
	height := v.contentHeight(viewport)
	for distance := 0; distance < height; distance++ {
		for _, y := range []int{target + int(direction)*distance, target - int(direction)*distance} {
			if y < viewport.top || y >= viewport.top+height || y >= len(v.rows) {
				continue
			}
			if cursor, ok := v.cursorAt(y, pane); ok {
				return cursor, true
			}
		}
	}
	return diffCursor{}, false
}

func (v *diffView) maxHorizontalOffset(width int) int {
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
		longest = max(longest, lipgloss.Width(expandTabs(ansi.Strip(current.text+current.left+current.right)))+extra)
	}
	return max(0, longest-contentWidth)
}
