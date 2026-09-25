package ui

import "github.com/charmbracelet/x/ansi"

func (v *diffView) NewViewport(width, height int) Viewport {
	return v.Resize(Viewport{}, width, height)
}

func (v *diffView) Resize(viewport Viewport, width, height int) Viewport {
	viewport.Width, viewport.Height = max(1, width), max(1, height)
	return v.clampViewport(viewport)
}

func (v *diffView) clampViewport(viewport Viewport) Viewport {
	maxTop := max(0, len(v.rows)-viewport.Height)
	if v.hasStickyHeader(Coordinate{Y: maxTop}, viewport.Height) {
		maxTop++
	}
	viewport.Top.Y = max(0, min(viewport.Top.Y, maxTop))
	viewport.LeftColumn = max(0, min(viewport.LeftColumn, v.maxHorizontalOffset(viewport.Width)))
	return viewport
}

func (v *diffView) hasStickyHeader(top Coordinate, viewportHeight int) bool {
	y, rows := top.Y, v.rows
	return viewportHeight > 1 && y >= 0 && y < len(rows) && rows[y].kind != fileRow
}

func (v *diffView) contentHeight(viewport Viewport) int {
	height := viewport.Height
	if v.hasStickyHeader(viewport.Top, height) {
		height--
	}
	return max(1, height)
}

func (v *diffView) KeepVisible(viewport Viewport, cursor Cursor) Viewport {
	clamp := v.clampViewport
	if !v.valid(cursor) {
		return clamp(viewport)
	}
	viewport = clamp(viewport)
	y := cursor.Coordinate.Y
	for range 2 {
		height := v.contentHeight(viewport)
		top := viewport.Top.Y
		if y < top {
			top = y
		}
		if y >= top+height {
			top = y - height + 1
		}
		viewport.Top.Y = top
		viewport = clamp(viewport)
	}
	return viewport
}

func (v *diffView) Align(viewport Viewport, cursor Cursor, alignment VerticalAlignment) Viewport {
	height := viewport.Height
	top := viewport.Top
	y := cursor.Coordinate.Y
	headerHeight := 0
	if height > 1 {
		headerHeight = 1
	}
	offset := max(0, alignmentOffset(height, alignment)-headerHeight)
	top.Y = y - offset
	if !v.hasStickyHeader(top, height) {
		top.Y = y - alignmentOffset(height, alignment)
	}
	viewport.Top = top
	return v.clampViewport(viewport)
}

func alignmentOffset(height int, alignment VerticalAlignment) int {
	if alignment == Middle {
		return height / 2
	}
	if alignment == Bottom {
		return height - 1
	}
	return 0
}

func (v *diffView) ScrollHorizontal(viewport Viewport, columns int) Viewport {
	viewport.LeftColumn += columns
	return v.clampViewport(viewport)
}

func (v *diffView) ScrollHalfPage(viewport Viewport, cursor Cursor, direction Direction) (Viewport, Cursor) {
	if !v.valid(cursor) {
		return viewport, cursor
	}
	distance := int(direction) * max(1, viewport.Height/2)
	viewport.Top.Y += distance
	viewport = v.clampViewport(viewport)
	target := min(len(v.rows)-1, max(0, cursor.Coordinate.Y+distance))
	if candidate, ok := v.nearest(target, cursor.Pane, direction, viewport); ok {
		cursor = candidate
	}
	return viewport, cursor
}

func (v *diffView) ViewportProgress(viewport Viewport) int {
	rows := v.rows
	if len(rows) == 0 {
		return 0
	}
	bottom := min(len(rows), viewport.Top.Y+v.contentHeight(viewport))
	return bottom * 100 / len(rows)
}

func (v *diffView) nearest(target int, pane Pane, direction Direction, viewport Viewport) (Cursor, bool) {
	height := v.contentHeight(viewport)
	top := viewport.Top.Y
	for distance := 0; distance < height; distance++ {
		for _, y := range []int{target + int(direction)*distance, target - int(direction)*distance} {
			if y < top || y >= top+height || y >= len(v.rows) {
				continue
			}
			if cursor, ok := v.cursorAt(y, pane); ok {
				return cursor, true
			}
		}
	}
	return Cursor{}, false
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
		longest = max(longest, widthOf(expandTabs(ansi.Strip(current.text+current.left+current.right)))+extra)
	}
	return max(0, longest-contentWidth)
}
