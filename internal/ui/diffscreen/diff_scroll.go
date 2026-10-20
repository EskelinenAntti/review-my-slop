package diffscreen

import "github.com/eskelinenantti/review-my-slop/internal/ui/internal/scroll"

func (v *diffView) scrollVertical(
	viewport diffViewport,
	cursor diffCursor,
	rows int,
) (diffViewport, diffCursor) {
	viewport.top = scroll.Offset(viewport.top, rows, v.maxTop(viewport.height))
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

func (v *diffView) scrollHalfPage(
	viewport diffViewport,
	cursor diffCursor,
	direction direction,
) (diffViewport, diffCursor) {
	if !v.valid(cursor) {
		return viewport, cursor
	}
	distance := int(direction) * max(1, viewport.height/2)
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
	if cursor, ok := v.cursorInViewport(target, pane, viewport); ok {
		return cursor, true
	}
	for y := range neighborRows(target, v.contentHeight(viewport), direction) {
		if cursor, ok := v.cursorInViewport(y, pane, viewport); ok {
			return cursor, true
		}
	}
	return diffCursor{}, false
}

func (v *diffView) cursorInViewport(
	y int,
	pane diffPane,
	viewport diffViewport,
) (diffCursor, bool) {
	if !v.rowVisible(viewport, y) {
		return diffCursor{}, false
	}
	return v.cursorAt(y, pane)
}
