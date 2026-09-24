package ui

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

func (v *diffView) valid(cursor Cursor) bool {
	return cursor.Coordinate >= 0 && cursor.Coordinate < len(v.rows) && v.lineIndex(v.rows[cursor.Coordinate], cursor.Pane) >= 0
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
	cursor := Cursor{y, pane}
	return cursor, v.valid(cursor)
}

func (v *diffView) First() (Cursor, bool) {
	if cursor, ok := v.scan(-1, Right, Forward, false); ok {
		return cursor, true
	}
	return v.scan(-1, Left, Forward, false)
}

func (v *diffView) scan(start int, pane Pane, direction Direction, wrap bool) (Cursor, bool) {
	last, y := len(v.rows)-1, start
	for range last + 1 {
		y += int(direction)
		if y < 0 || y > last {
			if !wrap {
				return Cursor{}, false
			}
			if y < 0 {
				y = last
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

func (v *diffView) Search(query string, cursor Cursor, direction Direction) (Cursor, bool) {
	if query == "" || !v.valid(cursor) {
		return Cursor{}, false
	}
	rows, split := v.rows, v.split
	lower, cursorAt := strings.ToLower, v.cursorAt
	query = lower(query)
	y, pane := cursor.Coordinate, cursor.Pane
	last := len(rows) - 1
	for range last {
		y += int(direction)
		if y < 0 {
			y = last
		}
		if y > last {
			y = 0
		}
		current := rows[y]
		for _, candidatePane := range []Pane{pane, Right - pane} {
			candidate, ok := cursorAt(y, candidatePane)
			if !ok || !split && candidatePane != pane {
				continue
			}
			line, _ := v.Line(candidate)
			if strings.Contains(lower(line.Text), query) {
				return candidate, true
			}
		}
		if current.kind != lineRow && strings.Contains(lower(ansi.Strip(current.text)), query) {
			for distance := range last + 1 {
				distance++
				offset := int(direction) * distance
				for _, candidateY := range []int{y + offset, y - offset} {
					if candidateY < 0 || candidateY > last || rows[candidateY].file != current.file {
						continue
					}
					if candidate, ok := cursorAt(candidateY, pane); ok {
						return candidate, true
					}
					if split {
						if candidate, ok := cursorAt(candidateY, Right-pane); ok {
							return candidate, true
						}
					}
				}
			}
		}
	}
	return Cursor{}, false
}

func (v *diffView) SwitchPane(cursor Cursor, pane Pane) (Cursor, bool) {
	rows, cursorAt, cursorY := v.rows, v.cursorAt, cursor.Coordinate
	if !v.split || !v.valid(cursor) {
		return Cursor{}, false
	}
	if candidate, ok := cursorAt(cursorY, pane); ok {
		return candidate, true
	}
	for y := cursorY - 1; y >= 0; y-- {
		if rows[y].file != rows[cursorY].file {
			break
		}
		if candidate, ok := cursorAt(y, pane); ok {
			return candidate, true
		}
	}
	for y := cursorY + 1; y < len(rows); y++ {
		if candidate, ok := cursorAt(y, pane); ok {
			return candidate, true
		}
	}
	return Cursor{}, false
}
