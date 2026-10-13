package diffscreen

func (v *diffView) scrollVertical(
	viewport diffViewport,
	cursor diffCursor,
	rows int,
) (diffViewport, diffCursor) {
	rows = max(-viewport.top, min(rows, v.maxTop(viewport.Height)-viewport.top))
	viewport.top += rows
	viewport = v.clampViewport(viewport)
	if !v.valid(cursor) || v.cursorVisible(viewport, cursor) {
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
	return v.valid(cursor) && cursor.row >= viewport.top &&
		cursor.row < viewport.top+v.contentHeight(viewport)
}

func (v *diffView) scrollHalfPage(
	viewport diffViewport,
	cursor diffCursor,
	direction direction,
) (diffViewport, diffCursor) {
	if !v.valid(cursor) {
		return viewport, cursor
	}
	distance := int(direction) * max(1, viewport.Height/2)
	viewport.top += distance
	viewport = v.clampViewport(viewport)
	target := min(len(v.rows)-1, max(0, cursor.row+distance))
	candidate, ok := v.nearest(target, cursor.pane, direction, viewport)
	if ok {
		cursor = candidate
	}
	return viewport, cursor
}

func (v *diffView) nearest(
	target int,
	pane diffPane,
	direction direction,
	viewport diffViewport,
) (diffCursor, bool) {
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
	return diffCursor{}, false
}

func (v *diffView) cursorInViewport(
	y int,
	pane diffPane,
	viewport diffViewport,
) (diffCursor, bool) {
	if y < viewport.top || y >= viewport.top+v.contentHeight(viewport) {
		return diffCursor{}, false
	}
	return v.cursorAt(y, pane)
}
