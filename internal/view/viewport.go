package view

import (
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

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
	return viewportHeight > 1 && top.Y >= 0 && top.Y < len(v.rows) && v.rows[top.Y].kind != fileRow
}

func (v *diffView) contentHeight(viewport Viewport) int {
	height := viewport.Height
	if v.hasStickyHeader(viewport.Top, viewport.Height) {
		height--
	}
	return max(1, height)
}

func (v *diffView) KeepVisible(viewport Viewport, cursor Cursor) Viewport {
	if !v.valid(cursor) {
		return v.clampViewport(viewport)
	}
	viewport = v.clampViewport(viewport)
	for range 2 {
		height := v.contentHeight(viewport)
		if cursor.Coordinate.Y < viewport.Top.Y {
			viewport.Top.Y = cursor.Coordinate.Y
		}
		if cursor.Coordinate.Y >= viewport.Top.Y+height {
			viewport.Top.Y = cursor.Coordinate.Y - height + 1
		}
		viewport = v.clampViewport(viewport)
	}
	return viewport
}

func (v *diffView) Align(viewport Viewport, cursor Cursor, alignment VerticalAlignment) Viewport {
	headerHeight := 0
	if viewport.Height > 1 {
		headerHeight = 1
	}
	offset := max(0, alignmentOffset(viewport.Height, alignment)-headerHeight)
	viewport.Top.Y = cursor.Coordinate.Y - offset
	if !v.hasStickyHeader(viewport.Top, viewport.Height) {
		viewport.Top.Y = cursor.Coordinate.Y - alignmentOffset(viewport.Height, alignment)
	}
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
	if len(v.rows) == 0 {
		return 0
	}
	bottom := min(len(v.rows), viewport.Top.Y+v.contentHeight(viewport))
	return bottom * 100 / len(v.rows)
}

func (v *diffView) nearest(target int, pane Pane, direction Direction, viewport Viewport) (Cursor, bool) {
	height := v.contentHeight(viewport)
	for distance := 0; distance < height; distance++ {
		for _, y := range []int{target + int(direction)*distance, target - int(direction)*distance} {
			if y < viewport.Top.Y || y >= viewport.Top.Y+height || y >= len(v.rows) {
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
		longest = max(longest, lipgloss.Width(expandTabs(ansi.Strip(current.text+current.left+current.right)))+extra)
	}
	return max(0, longest-contentWidth)
}
