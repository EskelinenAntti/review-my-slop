package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

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
	contentWidth, extra := max(1, viewport.Width-14), 0
	if v.split {
		contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
	}
	longest := 0
	for _, current := range v.rows {
		if current.kind == lineRow {
			longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
		}
	}
	viewport.LeftColumn = max(0, min(viewport.LeftColumn, max(0, longest-contentWidth)))
	return viewport
}

func (v *diffView) hasStickyHeader(top int, viewportHeight int) bool {
	return viewportHeight > 1 && top >= 0 && top < len(v.rows) && v.rows[top].kind != fileRow
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
	if v.valid(cursor) {
		viewport = v.clampViewport(viewport)
		for range 2 {
			height, top := v.contentHeight(viewport), viewport.Top
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
	return v.clampViewport(viewport)
}
