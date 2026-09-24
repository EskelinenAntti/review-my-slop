package ui

import "github.com/eskelinenantti/review-my-slop/internal/patch"

func (v *diffView) NewViewport(width, height int) Viewport {
	return v.Resize(Viewport{}, width, height)
}

func (v *diffView) ViewportProgress(viewport Viewport) int {
	if len(v.rows) == 0 {
		return 0
	}
	bottom := min(len(v.rows), viewport.Top+v.contentHeight(viewport))
	return bottom * 100 / len(v.rows)
}

func (v *diffView) ScrollHalfPage(viewport Viewport, cursor Cursor, direction Direction) (Viewport, Cursor) {
	if !v.valid(cursor) {
		return viewport, cursor
	}
	distance := int(direction) * max(1, viewport.Height/2)
	viewport.Top += distance
	viewport = v.clampViewport(viewport)
	target := min(len(v.rows)-1, max(0, cursor.Coordinate+distance))
	height := v.contentHeight(viewport)
	top := viewport.Top
	for distance := 0; distance < height; distance++ {
		offset := int(direction) * distance
		for _, y := range []int{target + offset, target - offset} {
			if y < top || y >= top+height || y >= len(v.rows) {
				continue
			}
			if candidate, ok := v.cursorAt(y, cursor.Pane); ok {
				return viewport, candidate
			}
		}
	}
	return viewport, cursor
}

func (v *diffView) ScrollHorizontal(viewport Viewport, columns int) Viewport {
	viewport.LeftColumn += columns
	return v.clampViewport(viewport)
}

func (v *diffView) Hunk(cursor Cursor) (patch.Hunk, bool) {
	if !v.valid(cursor) {
		return patch.Hunk{}, false
	}
	current := v.rows[cursor.Coordinate]
	return v.patch.Files[current.file].Hunks[current.hunk], true
}

func (v *diffView) Lines(selection Selection) []patch.Line {
	first, last := selection.First.Coordinate, selection.Last.Coordinate
	lines := v.selectedLines(selection)
	if first == last && selection.First.Pane != selection.Last.Pane && len(lines) > 1 && lines[0] == lines[1] {
		return lines[:1]
	}
	return lines
}

func (v *diffView) Last() (Cursor, bool) {
	if cursor, ok := v.scan(len(v.rows), Right, Backward); ok {
		return cursor, true
	}
	return v.scan(len(v.rows), Left, Backward)
}

func (v *diffView) Align(viewport Viewport, cursor Cursor, alignment VerticalAlignment) Viewport {
	height := viewport.Height
	headerHeight := 0
	if height > 1 {
		headerHeight = 1
	}
	alignmentOffset := 0
	switch alignment {
	case Middle:
		alignmentOffset = height / 2
	case Bottom:
		alignmentOffset = height - 1
	}
	offset := max(0, alignmentOffset-headerHeight)
	viewport.Top = cursor.Coordinate - offset
	if !v.hasStickyHeader(viewport.Top, height) {
		viewport.Top = cursor.Coordinate - alignmentOffset
	}
	return v.clampViewport(viewport)
}

func (v *diffView) JumpFile(cursor Cursor, direction Direction) (Cursor, bool) {
	if !v.valid(cursor) {
		return Cursor{}, false
	}
	file, _ := v.File(cursor)
	y := cursor.Coordinate
	for {
		next, ok := v.scan(y, cursor.Pane, direction)
		if !ok {
			return Cursor{}, false
		}
		nextFile, _ := v.File(next)
		if nextFile.OldPath != file.OldPath || nextFile.NewPath != file.NewPath {
			return next, true
		}
		y = next.Coordinate
	}
}

func (v *diffView) Move(cursor Cursor, direction Direction) (Cursor, bool) {
	if !v.valid(cursor) {
		return Cursor{}, false
	}
	return v.scan(cursor.Coordinate, cursor.Pane, direction)
}
