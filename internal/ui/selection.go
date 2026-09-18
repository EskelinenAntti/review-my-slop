package ui

import "github.com/eskelinenantti/review-my-slop/internal/diff"

func (p *presentation) BeginSelection(cursor Cursor) Selection {
	return Selection{First: cursor, Last: cursor}
}

func (p *presentation) ExtendSelection(selection Selection, cursor Cursor) (Selection, bool) {
	if !p.valid(selection.First) || !p.valid(cursor) {
		return selection, false
	}
	first := p.rows[selection.First.Coordinate.Y]
	last := p.rows[cursor.Coordinate.Y]
	if first.fileIndex != last.fileIndex || first.hunkIndex != last.hunkIndex {
		return selection, false
	}
	selection.Last = cursor
	return selection, true
}

// selectedLines is the one authoritative traversal used by both the comment
// anchor and any future selection presentation. It returns semantic diff lines
// rather than row numbers.
func (p *presentation) selectedLines(selection Selection) ([]diff.Line, diff.File, bool) {
	if _, ok := p.ExtendSelection(selection, selection.Last); !ok {
		return nil, diff.File{}, false
	}
	firstRow := p.rows[selection.First.Coordinate.Y]
	file := p.changes.Files[firstRow.fileIndex]
	low, high := selection.First, selection.Last
	if low.Coordinate.Y > high.Coordinate.Y {
		low, high = high, low
	}
	start, end := low.Coordinate.Y, high.Coordinate.Y
	lines := make([]diff.Line, 0, end-start+1)
	if start == end && selection.First.Pane != selection.Last.Pane {
		current := p.rows[start]
		for _, pane := range []Pane{selection.First.Pane, selection.Last.Pane} {
			index := p.lineIndex(current, pane)
			if index >= 0 {
				line := file.Hunks[current.hunkIndex].Lines[index]
				if len(lines) == 0 || lines[len(lines)-1] != line {
					lines = append(lines, line)
				}
			}
		}
		return lines, file, true
	}
	for y := start; y <= end; y++ {
		pane := low.Pane
		if y == high.Coordinate.Y {
			pane = high.Pane
		}
		if line, ok := p.line(Cursor{Coordinate: Coordinate{Y: y}, Pane: pane}); ok {
			lines = append(lines, line)
		}
	}
	return lines, file, true
}

func accumulateRange(start, end *int, value int) {
	if value == 0 {
		return
	}
	if *start == 0 || value < *start {
		*start = value
	}
	if value > *end {
		*end = value
	}
}
