package ui

import (
	"strings"

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
	if v.hasStickyHeader(maxTop, viewport.Height) {
		maxTop++
	}
	viewport.Top = max(0, min(viewport.Top, maxTop))
	viewport.LeftColumn = max(0, min(viewport.LeftColumn, v.maxHorizontalOffset(viewport.Width)))
	return viewport
}

func (v *diffView) hasStickyHeader(top int, viewportHeight int) bool {
	y := top
	return viewportHeight > 1 && y >= 0 && y < len(v.rows) && v.rows[y].kind != fileRow
}

func (v *diffView) contentHeight(viewport Viewport) int {
	height := viewport.Height
	if v.hasStickyHeader(viewport.Top, viewport.Height) {
		height--
	}
	return max(1, height)
}

func (v *diffView) KeepVisible(viewport Viewport, cursor Cursor) Viewport {
	cursorY := cursor.Coordinate
	if !v.valid(cursor) {
		return v.clampViewport(viewport)
	}
	viewport = v.clampViewport(viewport)
	for range 2 {
		height := v.contentHeight(viewport)
		top := viewport.Top
		if cursorY < top {
			top = cursorY
		}
		if cursorY >= top+height {
			top = cursorY - height + 1
		}
		viewport.Top = top
		viewport = v.clampViewport(viewport)
	}
	return viewport
}

func (v *diffView) Align(viewport Viewport, cursor Cursor, alignment VerticalAlignment) Viewport {
	height := viewport.Height
	headerHeight := 0
	if height > 1 {
		headerHeight = 1
	}
	offset := max(0, alignmentOffset(height, alignment)-headerHeight)
	viewport.Top = cursor.Coordinate - offset
	if !v.hasStickyHeader(viewport.Top, height) {
		viewport.Top = cursor.Coordinate - alignmentOffset(height, alignment)
	}
	return v.clampViewport(viewport)
}

func alignmentOffset(height int, alignment VerticalAlignment) int {
	switch alignment {
	case Middle:
		return height / 2
	case Bottom:
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
	viewport.Top += distance
	viewport = v.clampViewport(viewport)
	target := min(len(v.rows)-1, max(0, cursor.Coordinate+distance))
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
	bottom := min(len(rows), viewport.Top+v.contentHeight(viewport))
	return bottom * 100 / len(rows)
}

func (v *diffView) nearest(target int, pane Pane, direction Direction, viewport Viewport) (Cursor, bool) {
	height := v.contentHeight(viewport)
	top := viewport.Top
	for distance := 0; distance < height; distance++ {
		offset := int(direction) * distance
		for _, y := range []int{target + offset, target - offset} {
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
		longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
	}
	return max(0, longest-contentWidth)
}
