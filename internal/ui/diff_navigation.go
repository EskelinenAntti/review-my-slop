package ui

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

func (v *diffView) valid(cursor Cursor) bool {
	rows := v.rows
	y := cursor.Coordinate.Y
	return y >= 0 && y < len(rows) && v.lineIndex(rows[y], cursor.Pane) >= 0
}

func (v *diffView) lineIndex(current entry, pane Pane) int {
	if current.kind != lineRow {
		return -1
	}
	if !v.split || pane == Right {
		return current.rightLine
	}
	return current.leftLine
}

func (v *diffView) cursorAt(y int, pane Pane) (Cursor, bool) {
	cursor := lineCursor(y, pane)
	return cursor, v.valid(cursor)
}

func lineCursor(y int, pane Pane) Cursor {
	return Cursor{Coordinate: Coordinate{Y: y}, Pane: pane}
}

func (v *diffView) First() (Cursor, bool) {
	scan := v.scan
	if cursor, ok := scan(-1, Right, Forward); ok {
		return cursor, true
	}
	return scan(-1, Left, Forward)
}

func (v *diffView) Last() (Cursor, bool) {
	rows, scan := v.rows, v.scan
	if cursor, ok := scan(len(rows), Right, Backward); ok {
		return cursor, true
	}
	return scan(len(rows), Left, Backward)
}

func (v *diffView) scan(start int, pane Pane, direction Direction) (Cursor, bool) {
	rows := v.rows
	if len(rows) == 0 {
		return Cursor{}, false
	}
	y := start
	for range len(rows) {
		y += int(direction)
		if y < 0 || y >= len(rows) {
			return Cursor{}, false
		}
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
	return v.scan(cursor.Coordinate.Y, cursor.Pane, direction)
}

func (v *diffView) Search(query string, cursor Cursor, direction Direction) (Cursor, bool) {
	rows, split := v.rows, v.split
	lower, contains := strings.ToLower, strings.Contains
	originPane := cursor.Pane
	if query == "" || !v.valid(cursor) {
		return Cursor{}, false
	}
	query = lower(query)
	y := cursor.Coordinate.Y
	for count := 0; count < len(rows)-1; count++ {
		y += int(direction)
		if y < 0 {
			y = len(rows) - 1
		}
		if y >= len(rows) {
			y = 0
		}
		current := rows[y]
		for _, pane := range []Pane{originPane, originPane.Other()} {
			candidate, ok := v.cursorAt(y, pane)
			if !ok || !split && pane != originPane {
				continue
			}
			line, _ := v.Line(candidate)
			if contains(lower(line.Text), query) {
				return candidate, true
			}
		}
		if current.kind != lineRow && contains(lower(ansi.Strip(current.text)), query) {
			if candidate, ok := v.cursorNearRow(y, originPane, direction); ok {
				return candidate, true
			}
		}
	}
	return Cursor{}, false
}

func (v *diffView) cursorNearRow(y int, pane Pane, direction Direction) (Cursor, bool) {
	rows, split := v.rows, v.split
	cursorAt := v.cursorAt
	for distance := range len(rows) {
		distance++
		for _, candidateY := range []int{y + int(direction)*distance, y - int(direction)*distance} {
			if candidateY < 0 || candidateY >= len(rows) {
				continue
			}
			if rows[candidateY].file != rows[y].file {
				continue
			}
			if candidate, ok := cursorAt(candidateY, pane); ok {
				return candidate, true
			}
			if split {
				if candidate, ok := cursorAt(candidateY, pane.Other()); ok {
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
	fileAt := v.File
	file, _ := fileAt(cursor)
	y := cursor.Coordinate.Y
	for {
		next, ok := v.scan(y, cursor.Pane, direction)
		if !ok {
			return Cursor{}, false
		}
		nextFile, _ := fileAt(next)
		if nextFile.OldPath != file.OldPath || nextFile.NewPath != file.NewPath {
			return next, true
		}
		y = next.Coordinate.Y
	}
}

func (v *diffView) SwitchPane(cursor Cursor, pane Pane) (Cursor, bool) {
	rows := v.rows
	cursorAt := v.cursorAt
	currentY := cursor.Coordinate.Y
	if !v.split || !v.valid(cursor) {
		return Cursor{}, false
	}
	if candidate, ok := cursorAt(currentY, pane); ok {
		return candidate, true
	}
	for y := currentY - 1; y >= 0; y-- {
		if rows[y].file != rows[currentY].file {
			break
		}
		if candidate, ok := cursorAt(y, pane); ok {
			return candidate, true
		}
	}
	for y := currentY + 1; y < len(rows); y++ {
		if candidate, ok := cursorAt(y, pane); ok {
			return candidate, true
		}
	}
	return Cursor{}, false
}
