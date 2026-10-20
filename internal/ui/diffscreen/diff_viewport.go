package diffscreen

func (v *diffView) newViewport(width, height int) diffViewport {
	return v.resize(diffViewport{}, width, height)
}

func (v *diffView) resize(
	viewport diffViewport,
	width, height int,
) diffViewport {
	viewport.width, viewport.height = max(1, width), max(1, height)
	return v.clampViewport(viewport)
}

func (v *diffView) maxTop(height int) int {
	end := max(0, len(v.rows)-height)
	if v.hasStickyHeader(end, height) {
		end++
	}
	return end
}

func (v *diffView) clampViewport(viewport diffViewport) diffViewport {
	viewport.top = max(0, min(viewport.top, v.maxTop(viewport.height)))
	viewport.leftColumn = max(
		0,
		min(viewport.leftColumn, v.maxHorizontalOffset(viewport.width)),
	)
	return viewport
}

func (v *diffView) hasStickyHeader(top int, viewportHeight int) bool {
	return viewportHeight > 1 && top >= 0 &&
		top < len(v.rows) && v.rows[top].kind != fileRow
}

func (v *diffView) contentHeight(viewport diffViewport) int {
	height := viewport.height
	if v.hasStickyHeader(viewport.top, viewport.height) {
		height--
	}
	return max(1, height)
}

// contentEnd is the exclusive projected-row bound, excluding padding and
// accounting for the row reserved by a sticky header.
func (v *diffView) contentEnd(viewport diffViewport) int {
	return min(len(v.rows), viewport.top+v.contentHeight(viewport))
}

// rowAt maps a body cell to a projected row, excluding the sticky header and
// padding below the patch. Body coordinates do not include the screen frame.
func (v *diffView) rowAt(viewport diffViewport, bodyRow int) (int, bool) {
	if bodyRow < 0 || bodyRow >= viewport.height {
		return 0, false
	}
	if v.hasStickyHeader(viewport.top, viewport.height) {
		bodyRow--
	}
	row := viewport.top + bodyRow
	if bodyRow < 0 || row < 0 || row >= len(v.rows) {
		return 0, false
	}
	return row, true
}

func (v *diffView) keepVisible(
	viewport diffViewport,
	cursor diffCursor,
) diffViewport {
	if !v.valid(cursor) {
		return v.clampViewport(viewport)
	}
	viewport = v.clampViewport(viewport)
	// The top row may toggle the sticky header, changing usable height.
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

func (v *diffView) align(
	viewport diffViewport,
	cursor diffCursor,
	alignment Alignment,
) diffViewport {
	headerHeight := 0
	if viewport.height > 1 {
		headerHeight = 1
	}
	offset := max(0, alignmentOffset(viewport.height, alignment)-headerHeight)
	viewport.top = cursor.row - offset
	if !v.hasStickyHeader(viewport.top, viewport.height) {
		viewport.top = cursor.row - alignmentOffset(viewport.height, alignment)
	}
	return v.clampViewport(viewport)
}

func alignmentOffset(height int, alignment Alignment) int {
	if alignment == Center {
		return height / 2
	}
	if alignment == Bottom {
		return height - 1
	}
	return 0
}

func (v *diffView) cursorVisible(
	viewport diffViewport,
	cursor diffCursor,
) bool {
	return v.valid(cursor) && v.rowVisible(viewport, cursor.row)
}

func (v *diffView) viewportProgress(viewport diffViewport) int {
	if len(v.rows) == 0 {
		return 0
	}
	return v.contentEnd(viewport) * 100 / len(v.rows)
}

func (v *diffView) rowVisible(viewport diffViewport, row int) bool {
	return row >= viewport.top &&
		row < viewport.top+v.contentHeight(viewport)
}
