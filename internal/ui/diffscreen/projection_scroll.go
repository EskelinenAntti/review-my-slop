package diffscreen

func (v *patchProjection) scrollVertical(
	viewport patchViewport,
	cursor patchCursor,
	rows int,
) (patchViewport, patchCursor) {
	rows = max(
		-viewport.top,
		min(rows, v.maxViewportTop(viewport.Height)-viewport.top),
	)
	viewport.top += rows
	viewport = v.clampViewport(viewport)
	if !v.validCursor(cursor) || v.cursorVisibleInViewport(viewport, cursor) {
		return viewport, cursor
	}
	target := max(
		viewport.top,
		min(cursor.row, viewport.top+v.contentHeight(viewport)-1),
	)
	d := forward
	if cursor.row > target {
		d = backward
	}
	candidate, ok := v.nearestCursorInViewport(target, cursor.pane, d, viewport)
	if !ok && v.split {
		candidate, ok = v.nearestCursorInViewport(
			target, cursor.pane.other(), d, viewport,
		)
	}
	if ok {
		cursor = candidate
	}
	return viewport, cursor
}

func (v *patchProjection) cursorVisibleInViewport(
	viewport patchViewport,
	cursor patchCursor,
) bool {
	return v.validCursor(cursor) && cursor.row >= viewport.top &&
		cursor.row < viewport.top+v.contentHeight(viewport)
}

func (v *patchProjection) scrollHalfPage(
	viewport patchViewport,
	cursor patchCursor,
	direction direction,
) (patchViewport, patchCursor) {
	if !v.validCursor(cursor) {
		return viewport, cursor
	}
	distance := int(direction) * max(1, viewport.Height/2)
	viewport.top += distance
	viewport = v.clampViewport(viewport)
	target := min(len(v.rows)-1, max(0, cursor.row+distance))
	candidate, ok := v.nearestCursorInViewport(
		target, cursor.pane, direction, viewport,
	)
	if ok {
		cursor = candidate
	}
	return viewport, cursor
}

func (v *patchProjection) nearestCursorInViewport(
	target int,
	pane patchPane,
	direction direction,
	viewport patchViewport,
) (patchCursor, bool) {
	height := v.contentHeight(viewport)
	for distance := 0; distance < height; distance++ {
		for _, y := range []int{
			target + int(direction)*distance,
			target - int(direction)*distance,
		} {
			if cursor, ok := v.cursorInViewport(y, pane, viewport); ok {
				return cursor, true
			}
		}
	}
	return patchCursor{}, false
}

func (v *patchProjection) cursorInViewport(
	y int,
	pane patchPane,
	viewport patchViewport,
) (patchCursor, bool) {
	if y < viewport.top || y >= viewport.top+v.contentHeight(viewport) {
		return patchCursor{}, false
	}
	return v.cursorOnRow(y, pane)
}
