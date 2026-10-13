package diffscreen

import (
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func (v *patchProjection) newViewport(width, height int) patchViewport {
	return v.resize(patchViewport{}, width, height)
}

func (v *patchProjection) resize(
	viewport patchViewport,
	width, height int,
) patchViewport {
	viewport.Width, viewport.Height = max(1, width), max(1, height)
	return v.clampViewport(viewport)
}

func (v *patchProjection) maxViewportTop(height int) int {
	end := max(0, len(v.rows)-height)
	if v.showsStickyFileHeader(end, height) {
		end++
	}
	return end
}

func (v *patchProjection) clampViewport(viewport patchViewport) patchViewport {
	viewport.top = max(0, min(viewport.top, v.maxViewportTop(viewport.Height)))
	viewport.LeftColumn = max(
		0,
		min(viewport.LeftColumn, v.maxHorizontalOffset(viewport.Width)),
	)
	return viewport
}

func (v *patchProjection) showsStickyFileHeader(
	top int,
	viewportHeight int,
) bool {
	return viewportHeight > 1 && top >= 0 &&
		top < len(v.rows) && v.rows[top].kind != fileRow
}

func (v *patchProjection) contentHeight(viewport patchViewport) int {
	height := viewport.Height
	if v.showsStickyFileHeader(viewport.top, viewport.Height) {
		height--
	}
	return max(1, height)
}

func (v *patchProjection) keepCursorVisible(
	viewport patchViewport,
	cursor patchCursor,
) patchViewport {
	if !v.validCursor(cursor) {
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

func (v *patchProjection) alignCursor(
	viewport patchViewport,
	cursor patchCursor,
	alignment verticalAlignment,
) patchViewport {
	headerHeight := 0
	if viewport.Height > 1 {
		headerHeight = 1
	}
	offset := max(0, alignmentOffset(viewport.Height, alignment)-headerHeight)
	viewport.top = cursor.row - offset
	if !v.showsStickyFileHeader(viewport.top, viewport.Height) {
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

func (v *patchProjection) scrollHorizontal(
	viewport patchViewport,
	columns int,
) patchViewport {
	columns = max(
		-viewport.LeftColumn,
		min(columns, v.maxHorizontalOffset(viewport.Width)-viewport.LeftColumn),
	)
	viewport.LeftColumn += columns
	return v.clampViewport(viewport)
}

func (v *patchProjection) viewportProgress(viewport patchViewport) int {
	if len(v.rows) == 0 {
		return 0
	}
	bottom := min(len(v.rows), viewport.top+v.contentHeight(viewport))
	return bottom * 100 / len(v.rows)
}

func (v *patchProjection) maxHorizontalOffset(width int) int {
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
