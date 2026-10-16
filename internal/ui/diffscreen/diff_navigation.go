package diffscreen

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

func (v *diffView) isChangedLine(cursor diffCursor) bool {
	return cursor.row >= 0 && cursor.row < len(v.rows) &&
		v.findLinePositionInChangeBlock(v.rows[cursor.row], cursor.pane) >= 0
}

func (v *diffView) findLinePositionInChangeBlock(
	current entry,
	pane diffPane,
) int {
	if current.kind != lineRow {
		return -1
	}
	if !v.split || pane == right {
		return current.rightLine
	}
	return current.leftLine
}

func (v *diffView) changedLineAtRow(y int, pane diffPane) (diffCursor, bool) {
	cursor := diffCursor{row: y, pane: pane}
	return cursor, v.isChangedLine(cursor)
}

func (v *diffView) firstSelectableLine() (diffCursor, bool) {
	if cursor, ok := v.findFirstChangedLineInDirection(-1, right, forward); ok {
		return cursor, true
	}
	return v.findFirstChangedLineInDirection(-1, left, forward)
}

func (v *diffView) lastSelectableLine() (diffCursor, bool) {
	cursor, ok := v.findFirstChangedLineInDirection(
		len(v.rows), right, backward,
	)
	if ok {
		return cursor, true
	}
	return v.findFirstChangedLineInDirection(len(v.rows), left, backward)
}

func (v *diffView) findFirstChangedLineInDirection(
	start int,
	pane diffPane,
	direction direction,
) (diffCursor, bool) {
	lineStep := int(direction)
	for y := start + lineStep; y >= 0 && y < len(v.rows); y += lineStep {
		if cursor, ok := v.changedLineAtRow(y, pane); ok {
			return cursor, true
		}
	}
	return diffCursor{}, false
}

func (v *diffView) moveInDirection(
	cursor diffCursor,
	direction direction,
) (diffCursor, bool) {
	if !v.isChangedLine(cursor) {
		return diffCursor{}, false
	}
	return v.findFirstChangedLineInDirection(cursor.row, cursor.pane, direction)
}

func (v *diffView) findTextInChanges(
	query string,
	cursor diffCursor,
	direction direction,
) (diffCursor, bool) {
	if query == "" || !v.isChangedLine(cursor) {
		return diffCursor{}, false
	}
	query = strings.ToLower(query)
	y := cursor.row
	for count := 0; count < len(v.rows)-1; count++ {
		y = (y + int(direction) + len(v.rows)) % len(v.rows)
		candidate, ok := v.findTextOnLine(
			query, y, cursor.pane, direction,
		)
		if ok {
			return candidate, true
		}
	}
	return diffCursor{}, false
}

func (v *diffView) findTextOnLine(
	query string,
	y int,
	pane diffPane,
	direction direction,
) (diffCursor, bool) {
	if v.rows[y].kind != lineRow {
		return v.findTextInFileDetails(query, y, pane, direction)
	}
	panes := []diffPane{pane}
	if v.split {
		panes = append(panes, pane.oppositePane())
	}
	for _, candidatePane := range panes {
		candidate, ok := v.changedLineAtRow(y, candidatePane)
		if !ok {
			continue
		}
		line, _ := v.patchLineForPosition(candidate)
		if strings.Contains(strings.ToLower(line.Text), query) {
			return candidate, true
		}
	}
	return diffCursor{}, false
}

func (v *diffView) findTextInFileDetails(
	query string,
	y int,
	pane diffPane,
	direction direction,
) (diffCursor, bool) {
	if strings.Contains(strings.ToLower(ansi.Strip(v.rows[y].text)), query) {
		return v.findChangedLineNearRow(y, pane, direction)
	}
	return diffCursor{}, false
}

func (v *diffView) findChangedLineNearRow(
	y int,
	pane diffPane,
	direction direction,
) (diffCursor, bool) {
	for distance := 1; distance <= len(v.rows); distance++ {
		for _, candidateY := range []int{
			y + int(direction)*distance,
			y - int(direction)*distance,
		} {
			candidate, ok := v.findChangedLineInFile(
				candidateY, y, pane,
			)
			if ok {
				return candidate, true
			}
		}
	}
	return diffCursor{}, false
}

func (v *diffView) findChangedLineInFile(
	y, origin int,
	pane diffPane,
) (diffCursor, bool) {
	if y < 0 || y >= len(v.rows) || v.rows[y].file != v.rows[origin].file {
		return diffCursor{}, false
	}
	if candidate, ok := v.changedLineAtRow(y, pane); ok {
		return candidate, true
	}
	if v.split {
		return v.changedLineAtRow(y, pane.oppositePane())
	}
	return diffCursor{}, false
}

func (v *diffView) moveFocusToAdjacentFile(
	cursor diffCursor,
	direction direction,
) (diffCursor, bool) {
	if !v.isChangedLine(cursor) {
		return diffCursor{}, false
	}
	file, _ := v.fileForLine(cursor)
	y := cursor.row
	for {
		next, ok := v.findFirstChangedLineInDirection(y, cursor.pane, direction)
		if !ok {
			return diffCursor{}, false
		}
		nextFile, _ := v.fileForLine(next)
		if nextFile.OldPath != file.OldPath ||
			nextFile.NewPath != file.NewPath {
			return next, true
		}
		y = next.row
	}
}

func (v *diffView) switchToPane(
	cursor diffCursor,
	pane diffPane,
) (diffCursor, bool) {
	if !v.split || !v.isChangedLine(cursor) {
		return diffCursor{}, false
	}
	if candidate, ok := v.changedLineAtRow(cursor.row, pane); ok {
		return candidate, true
	}
	if candidate, ok := v.previousLineInFile(cursor, pane); ok {
		return candidate, true
	}
	return v.findFirstChangedLineInDirection(cursor.row, pane, forward)
}

func (v *diffView) previousLineInFile(
	cursor diffCursor,
	pane diffPane,
) (diffCursor, bool) {
	for y := cursor.row - 1; y >= 0; y-- {
		if v.rows[y].file != v.rows[cursor.row].file {
			break
		}
		if candidate, ok := v.changedLineAtRow(y, pane); ok {
			return candidate, true
		}
	}
	return diffCursor{}, false
}
