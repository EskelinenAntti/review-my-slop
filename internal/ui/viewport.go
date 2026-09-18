package ui

import (
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func (p *presentation) NewViewport(width, height int) Viewport {
	return p.Resize(Viewport{}, width, height)
}

func (p *presentation) Resize(viewport Viewport, width, height int) Viewport {
	viewport.Width, viewport.Height = max(1, width), max(1, height)
	return p.clampViewport(viewport)
}

func (p *presentation) clampViewport(viewport Viewport) Viewport {
	maxTop := max(0, len(p.rows)-viewport.Height)
	if p.hasStickyHeader(Coordinate{Y: maxTop}, viewport.Height) {
		maxTop++
	}
	viewport.Top.Y = max(0, min(viewport.Top.Y, maxTop))
	viewport.LeftColumn = max(0, min(viewport.LeftColumn, p.maxHorizontalOffset(viewport.Width)))
	return viewport
}

func (p *presentation) hasStickyHeader(top Coordinate, viewportHeight int) bool {
	return viewportHeight > 1 && top.Y >= 0 && top.Y < len(p.rows) && p.rows[top.Y].kind != fileRow
}

func (p *presentation) contentHeight(viewport Viewport) int {
	height := viewport.Height
	if p.hasStickyHeader(viewport.Top, viewport.Height) {
		height--
	}
	return max(1, height)
}

func (p *presentation) KeepVisible(viewport Viewport, cursor Cursor) Viewport {
	if !p.valid(cursor) {
		return p.clampViewport(viewport)
	}
	viewport = p.clampViewport(viewport)
	for range 2 {
		height := p.contentHeight(viewport)
		if cursor.Coordinate.Y < viewport.Top.Y {
			viewport.Top.Y = cursor.Coordinate.Y
		}
		if cursor.Coordinate.Y >= viewport.Top.Y+height {
			viewport.Top.Y = cursor.Coordinate.Y - height + 1
		}
		viewport = p.clampViewport(viewport)
	}
	return viewport
}

func (p *presentation) Align(viewport Viewport, cursor Cursor, alignment VerticalAlignment) Viewport {
	headerHeight := 0
	if viewport.Height > 1 {
		headerHeight = 1
	}
	offset := max(0, alignmentOffset(viewport.Height, alignment)-headerHeight)
	viewport.Top.Y = cursor.Coordinate.Y - offset
	if !p.hasStickyHeader(viewport.Top, viewport.Height) {
		viewport.Top.Y = cursor.Coordinate.Y - alignmentOffset(viewport.Height, alignment)
	}
	return p.clampViewport(viewport)
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

func (p *presentation) ScrollHorizontal(viewport Viewport, columns int) Viewport {
	viewport.LeftColumn += columns
	return p.clampViewport(viewport)
}

func (p *presentation) ScrollHalfPage(viewport Viewport, cursor Cursor, direction Direction) (Viewport, Cursor) {
	if !p.valid(cursor) {
		return viewport, cursor
	}
	distance := int(direction) * max(1, viewport.Height/2)
	viewport.Top.Y += distance
	viewport = p.clampViewport(viewport)
	target := min(len(p.rows)-1, max(0, cursor.Coordinate.Y+distance))
	if candidate, ok := p.nearest(target, cursor.Pane, direction, viewport); ok {
		cursor = candidate
	}
	return viewport, cursor
}

func (p *presentation) Progress(viewport Viewport) int {
	if len(p.rows) == 0 {
		return 0
	}
	bottom := min(len(p.rows), viewport.Top.Y+p.contentHeight(viewport))
	return bottom * 100 / len(p.rows)
}

func (p *presentation) nearest(target int, pane Pane, direction Direction, viewport Viewport) (Cursor, bool) {
	height := p.contentHeight(viewport)
	for distance := 0; distance < height; distance++ {
		for _, y := range []int{target + int(direction)*distance, target - int(direction)*distance} {
			if y < viewport.Top.Y || y >= viewport.Top.Y+height || y >= len(p.rows) {
				continue
			}
			if cursor, ok := p.cursorAt(y, pane); ok {
				return cursor, true
			}
		}
	}
	return Cursor{}, false
}

func (p *presentation) maxHorizontalOffset(width int) int {
	contentWidth := max(1, width-14)
	extra := 0
	if p.split {
		contentWidth, extra = max(1, (width-3)/2-6), 2
	}
	longest := 0
	for _, current := range p.rows {
		if current.kind != lineRow {
			continue
		}
		longest = max(longest, lipgloss.Width(expandTabs(ansi.Strip(current.text+current.left+current.right)))+extra)
	}
	return max(0, longest-contentWidth)
}
