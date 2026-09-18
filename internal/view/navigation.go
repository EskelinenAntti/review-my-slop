package view

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

func (v *diffView) valid(cursor Cursor) bool {
	_, ok := v.rowAt(cursor)
	return ok
}

func (v *diffView) lineIndex(current row, pane Pane) int {
	if current.kind != lineRow {
		return -1
	}
	if !v.split || pane == Right {
		return current.rightIndex
	}
	return current.leftIndex
}

func (v *diffView) cursorAt(y int, pane Pane) (Cursor, bool) {
	cursor := Cursor{Coordinate: Coordinate{Y: y}, Pane: pane}
	return cursor, v.valid(cursor)
}

func (v *diffView) First() (Cursor, bool) {
	if cursor, ok := v.findRow(-1, Right, Forward); ok {
		return cursor, true
	}
	return v.findRow(-1, Left, Forward)
}

func (v *diffView) Last() (Cursor, bool) {
	if cursor, ok := v.findRow(len(v.rows), Right, Backward); ok {
		return cursor, true
	}
	return v.findRow(len(v.rows), Left, Backward)
}

func (v *diffView) findRow(startY int, pane Pane, direction Direction) (Cursor, bool) {
	if len(v.rows) == 0 {
		return Cursor{}, false
	}
	for y := startY + int(direction); y >= 0 && y < len(v.rows); y += int(direction) {
		if cursor, ok := v.cursorAt(y, pane); ok {
			return cursor, true
		}
	}
	return Cursor{}, false
}

func (v *diffView) Move(cursor Cursor, direction Direction) (Cursor, bool) {
	if !v.valid(cursor) {
		return Cursor{}, false
	}
	return v.findRow(cursor.Coordinate.Y, cursor.Pane, direction)
}

func (v *diffView) Search(query string, cursor Cursor, direction Direction) (Cursor, bool) {
	if query == "" || !v.valid(cursor) {
		return Cursor{}, false
	}
	query = strings.ToLower(query)
	y := cursor.Coordinate.Y
	for steps := 0; steps < len(v.rows)-1; steps++ {
		y += int(direction)
		y = wrapRow(y, len(v.rows))
		if candidate, ok := v.searchLine(y, cursor.Pane, query); ok {
			return candidate, true
		}
		row := v.rows[y]
		if row.kind != lineRow && strings.Contains(strings.ToLower(ansi.Strip(row.text)), query) {
			if candidate, ok := v.nearestLineInFile(y, cursor.Pane, direction); ok {
				return candidate, true
			}
		}
	}
	return Cursor{}, false
}

func wrapRow(y, rowCount int) int {
	if y < 0 {
		return rowCount - 1
	}
	if y >= rowCount {
		return 0
	}
	return y
}

func (v *diffView) searchLine(y int, pane Pane, query string) (Cursor, bool) {
	panes := []Pane{pane}
	if v.split {
		panes = append(panes, pane.Other())
	}
	for _, candidatePane := range panes {
		candidate, ok := v.cursorAt(y, candidatePane)
		if !ok {
			continue
		}
		line, ok := v.Line(candidate)
		if !ok {
			continue
		}
		if strings.Contains(strings.ToLower(line.Text), query) {
			return candidate, true
		}
	}
	return Cursor{}, false
}

func (v *diffView) nearestLineInFile(y int, pane Pane, direction Direction) (Cursor, bool) {
	fileIndex := v.rows[y].fileIndex
	for distance := 1; distance <= len(v.rows); distance++ {
		for _, candidateY := range []int{y + int(direction)*distance, y - int(direction)*distance} {
			if candidateY < 0 || candidateY >= len(v.rows) {
				continue
			}
			if v.rows[candidateY].fileIndex != fileIndex {
				continue
			}
			if candidate, ok := v.cursorAt(candidateY, pane); ok {
				return candidate, true
			}
			if v.split {
				if candidate, ok := v.cursorAt(candidateY, pane.Other()); ok {
					return candidate, true
				}
			}
		}
	}
	return Cursor{}, false
}

func (v *diffView) JumpFile(cursor Cursor, direction Direction) (Cursor, bool) {
	if !v.valid(cursor) {
		return Cursor{}, false
	}
	file, ok := v.File(cursor)
	if !ok {
		return Cursor{}, false
	}
	y := cursor.Coordinate.Y
	for {
		next, ok := v.findRow(y, cursor.Pane, direction)
		if !ok {
			return Cursor{}, false
		}
		nextFile, ok := v.File(next)
		if !ok {
			return Cursor{}, false
		}
		if !sameFilePath(nextFile, file) {
			return next, true
		}
		y = next.Coordinate.Y
	}
}

func (v *diffView) SwitchPane(cursor Cursor, pane Pane) (Cursor, bool) {
	if !v.split || !v.valid(cursor) {
		return Cursor{}, false
	}
	if candidate, ok := v.cursorAt(cursor.Coordinate.Y, pane); ok {
		return candidate, true
	}
	fileIndex := v.rows[cursor.Coordinate.Y].fileIndex
	for _, direction := range []Direction{Backward, Forward} {
		if candidate, ok := v.findPaneInFile(cursor.Coordinate.Y, fileIndex, pane, direction); ok {
			return candidate, true
		}
	}
	return Cursor{}, false
}

func (v *diffView) findPaneInFile(startY, fileIndex int, pane Pane, direction Direction) (Cursor, bool) {
	for y := startY + int(direction); y >= 0 && y < len(v.rows); y += int(direction) {
		if v.rows[y].fileIndex != fileIndex {
			return Cursor{}, false
		}
		if candidate, ok := v.cursorAt(y, pane); ok {
			return candidate, true
		}
	}
	return Cursor{}, false
}
