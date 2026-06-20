package view

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

func (v *diffView) valid(cursor Cursor) bool {
	return cursor.Coordinate.Y >= 0 && cursor.Coordinate.Y < len(v.rows) && v.lineIndex(v.rows[cursor.Coordinate.Y], cursor.Pane) >= 0
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
	cursor := Cursor{Coordinate: Coordinate{Y: y}, Pane: pane}
	return cursor, v.valid(cursor)
}

func (v *diffView) First() (Cursor, bool) {
	if cursor, ok := v.scan(-1, Right, Forward, false); ok {
		return cursor, true
	}
	return v.scan(-1, Left, Forward, false)
}

func (v *diffView) Last() (Cursor, bool) {
	if cursor, ok := v.scan(len(v.rows), Right, Backward, false); ok {
		return cursor, true
	}
	return v.scan(len(v.rows), Left, Backward, false)
}

func (v *diffView) scan(start int, pane Pane, direction Direction, wrap bool) (Cursor, bool) {
	if len(v.rows) == 0 {
		return Cursor{}, false
	}
	y := start
	for count := 0; count < len(v.rows); count++ {
		y += int(direction)
		if y < 0 || y >= len(v.rows) {
			if !wrap {
				return Cursor{}, false
			}
			if y < 0 {
				y = len(v.rows) - 1
			} else {
				y = 0
			}
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
	return v.scan(cursor.Coordinate.Y, cursor.Pane, direction, false)
}

func (v *diffView) Search(query string, cursor Cursor, direction Direction) (Cursor, bool) {
	if query == "" || !v.valid(cursor) {
		return Cursor{}, false
	}
	query = strings.ToLower(query)
	y := cursor.Coordinate.Y
	for count := 0; count < len(v.rows)-1; count++ {
		y += int(direction)
		if y < 0 {
			y = len(v.rows) - 1
		}
		if y >= len(v.rows) {
			y = 0
		}
		current := v.rows[y]
		for _, pane := range []Pane{cursor.Pane, cursor.Pane.Other()} {
			candidate, ok := v.cursorAt(y, pane)
			if !ok || !v.split && pane != cursor.Pane {
				continue
			}
			line, _ := v.Line(candidate)
			if strings.Contains(strings.ToLower(line.Text), query) {
				return candidate, true
			}
		}
		if current.kind != lineRow && strings.Contains(strings.ToLower(ansi.Strip(current.text)), query) {
			if candidate, ok := v.cursorNearRow(y, cursor.Pane, direction); ok {
				return candidate, true
			}
		}
	}
	return Cursor{}, false
}

func (v *diffView) cursorNearRow(y int, pane Pane, direction Direction) (Cursor, bool) {
	for distance := 1; distance <= len(v.rows); distance++ {
		for _, candidateY := range []int{y + int(direction)*distance, y - int(direction)*distance} {
			if candidateY < 0 || candidateY >= len(v.rows) {
				continue
			}
			if v.rows[candidateY].file != v.rows[y].file {
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
	file, _ := v.File(cursor)
	y := cursor.Coordinate.Y
	for {
		next, ok := v.scan(y, cursor.Pane, direction, false)
		if !ok {
			return Cursor{}, false
		}
		nextFile, _ := v.File(next)
		if nextFile.OldPath != file.OldPath || nextFile.NewPath != file.NewPath {
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
	for y := cursor.Coordinate.Y - 1; y >= 0; y-- {
		if v.rows[y].file != v.rows[cursor.Coordinate.Y].file {
			break
		}
		if candidate, ok := v.cursorAt(y, pane); ok {
			return candidate, true
		}
	}
	for y := cursor.Coordinate.Y + 1; y < len(v.rows); y++ {
		if candidate, ok := v.cursorAt(y, pane); ok {
			return candidate, true
		}
	}
	return Cursor{}, false
}
