package diffscreen

func renderFocusedRow(
	v *patchProjection,
	cursor patchCursor,
	width int,
	selection *patchRange,
) string {
	return v.renderViewportAtRow(cursor, cursor, width, selection)
}

func (v *patchProjection) renderViewportAtRow(
	target, active patchCursor,
	width int,
	selection *patchRange,
) string {
	viewport := v.newViewport(width, 1)
	viewport.top = target.row
	return v.renderProjection(viewport, active, selection)
}
