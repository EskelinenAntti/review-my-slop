package ui

import (
	"strings"

	"github.com/charmbracelet/x/ansi"

	"github.com/eskelinenantti/review-my-slop/internal/diff"
)

func (p *presentation) buildUnified() {
	for fileIndex := range p.changes.Files {
		file := &p.changes.Files[fileIndex]
		p.rows = append(p.rows, row{kind: fileRow, fileIndex: fileIndex, hunkIndex: -1, leftLine: -1, rightLine: -1, text: visibleText(file.Path())})
		for _, metadata := range file.Metadata {
			p.rows = append(p.rows, row{kind: metadataRow, fileIndex: fileIndex, hunkIndex: -1, leftLine: -1, rightLine: -1, text: visibleText(metadata)})
		}
		highlighted := highlightSources(file.Path(), visibleSource(file.OldSource), visibleSource(file.NewSource), p.dark)
		for hunkIndex := range file.Hunks {
			hunk := &file.Hunks[hunkIndex]
			p.rows = append(p.rows, row{kind: hunkRow, fileIndex: fileIndex, hunkIndex: hunkIndex, leftLine: -1, rightLine: -1, text: hunkHeader(hunk.Header), hunkHeader: hunk.Header})
			for lineIndex, line := range hunk.Lines {
				text := visibleText(line.Text)
				if line.Kind == diff.Deletion {
					text = highlightedLine(highlighted.old, line.OldNumber, text)
				} else {
					text = highlightedLine(highlighted.new, line.NewNumber, text)
				}
				p.rows = append(p.rows, row{kind: lineRow, fileIndex: fileIndex, hunkIndex: hunkIndex, leftLine: lineIndex, rightLine: lineIndex, text: text, hunkHeader: hunk.Header})
			}
		}
	}
}

func (p *presentation) buildSplit() {
	for fileIndex := range p.changes.Files {
		file := &p.changes.Files[fileIndex]
		p.rows = append(p.rows, row{kind: fileRow, fileIndex: fileIndex, hunkIndex: -1, leftLine: -1, rightLine: -1, text: visibleText(file.Path())})
		for _, metadata := range file.Metadata {
			p.rows = append(p.rows, row{kind: metadataRow, fileIndex: fileIndex, hunkIndex: -1, leftLine: -1, rightLine: -1, text: visibleText(metadata)})
		}
		highlighted := highlightSources(file.Path(), visibleSource(file.OldSource), visibleSource(file.NewSource), p.dark)
		for hunkIndex := range file.Hunks {
			hunk := &file.Hunks[hunkIndex]
			p.rows = append(p.rows, row{kind: hunkRow, fileIndex: fileIndex, hunkIndex: hunkIndex, leftLine: -1, rightLine: -1, text: hunkHeader(hunk.Header), hunkHeader: hunk.Header})
			for index := 0; index < len(hunk.Lines); {
				line := hunk.Lines[index]
				switch line.Kind {
				case diff.Context:
					text := highlightedLine(highlighted.new, line.NewNumber, visibleText(line.Text))
					p.rows = append(p.rows, row{kind: lineRow, fileIndex: fileIndex, hunkIndex: hunkIndex, leftLine: index, rightLine: index, left: text, right: text, hunkHeader: hunk.Header})
					index++
				case diff.Addition:
					text := highlightedLine(highlighted.new, line.NewNumber, visibleText(line.Text))
					p.rows = append(p.rows, row{kind: lineRow, fileIndex: fileIndex, hunkIndex: hunkIndex, leftLine: -1, rightLine: index, right: text, hunkHeader: hunk.Header})
					index++
				case diff.Deletion:
					removedStart := index
					for index < len(hunk.Lines) && hunk.Lines[index].Kind == diff.Deletion {
						index++
					}
					addedStart, addedEnd := index, index
					for addedEnd < len(hunk.Lines) && hunk.Lines[addedEnd].Kind == diff.Addition {
						addedEnd++
					}
					count := max(index-removedStart, addedEnd-addedStart)
					for offset := 0; offset < count; offset++ {
						current := row{kind: lineRow, fileIndex: fileIndex, hunkIndex: hunkIndex, leftLine: -1, rightLine: -1, hunkHeader: hunk.Header}
						if removedStart+offset < index {
							current.leftLine = removedStart + offset
							old := hunk.Lines[current.leftLine]
							current.left = highlightedLine(highlighted.old, old.OldNumber, visibleText(old.Text))
						}
						if addedStart+offset < addedEnd {
							current.rightLine = addedStart + offset
							added := hunk.Lines[current.rightLine]
							current.right = highlightedLine(highlighted.new, added.NewNumber, visibleText(added.Text))
						}
						p.rows = append(p.rows, current)
					}
					index = addedEnd
				}
			}
		}
	}
}

func hunkHeader(header string) string {
	if strings.HasPrefix(header, "@@") {
		return visibleText(header)
	}
	return "@@ " + visibleText(header)
}

func (p *presentation) valid(cursor Cursor) bool {
	return cursor.Coordinate.Y >= 0 && cursor.Coordinate.Y < len(p.rows) && p.lineIndex(p.rows[cursor.Coordinate.Y], cursor.Pane) >= 0
}

func (p *presentation) lineIndex(current row, pane Pane) int {
	if current.kind != lineRow {
		return -1
	}
	if !p.split || pane == Right {
		return current.rightLine
	}
	return current.leftLine
}

func (p *presentation) cursorAt(y int, pane Pane) (Cursor, bool) {
	cursor := Cursor{Coordinate: Coordinate{Y: y}, Pane: pane}
	return cursor, p.valid(cursor)
}

func (p *presentation) First() (Cursor, bool) {
	if cursor, ok := p.scan(-1, Right, Forward, false); ok {
		return cursor, true
	}
	return p.scan(-1, Left, Forward, false)
}

func (p *presentation) Last() (Cursor, bool) {
	if cursor, ok := p.scan(len(p.rows), Right, Backward, false); ok {
		return cursor, true
	}
	return p.scan(len(p.rows), Left, Backward, false)
}

func (p *presentation) scan(start int, pane Pane, direction Direction, wrap bool) (Cursor, bool) {
	if len(p.rows) == 0 {
		return Cursor{}, false
	}
	y := start
	for count := 0; count < len(p.rows); count++ {
		y += int(direction)
		if y < 0 || y >= len(p.rows) {
			if !wrap {
				return Cursor{}, false
			}
			if y < 0 {
				y = len(p.rows) - 1
			} else {
				y = 0
			}
		}
		if cursor, ok := p.cursorAt(y, pane); ok {
			return cursor, true
		}
	}
	return Cursor{}, false
}

func (p *presentation) Move(cursor Cursor, direction Direction) (Cursor, bool) {
	if !p.valid(cursor) {
		return Cursor{}, false
	}
	return p.scan(cursor.Coordinate.Y, cursor.Pane, direction, false)
}

func (p *presentation) Search(query string, cursor Cursor, direction Direction) (Cursor, bool) {
	if query == "" || !p.valid(cursor) {
		return Cursor{}, false
	}
	query = strings.ToLower(query)
	y := cursor.Coordinate.Y
	for count := 0; count < len(p.rows)-1; count++ {
		y += int(direction)
		if y < 0 {
			y = len(p.rows) - 1
		}
		if y >= len(p.rows) {
			y = 0
		}
		current := p.rows[y]
		for _, pane := range []Pane{cursor.Pane, cursor.Pane.Other()} {
			candidate, ok := p.cursorAt(y, pane)
			if !ok || !p.split && pane != cursor.Pane {
				continue
			}
			line, _ := p.line(candidate)
			if strings.Contains(strings.ToLower(line.Text), query) {
				return candidate, true
			}
		}
		if current.kind != lineRow && strings.Contains(strings.ToLower(ansi.Strip(current.text)), query) {
			if candidate, ok := p.cursorNearRow(y, cursor.Pane, direction); ok {
				return candidate, true
			}
		}
	}
	return Cursor{}, false
}

func (p *presentation) cursorNearRow(y int, pane Pane, direction Direction) (Cursor, bool) {
	for distance := 1; distance <= len(p.rows); distance++ {
		for _, candidateY := range []int{y + int(direction)*distance, y - int(direction)*distance} {
			if candidateY < 0 || candidateY >= len(p.rows) || p.rows[candidateY].fileIndex != p.rows[y].fileIndex {
				continue
			}
			if candidate, ok := p.cursorAt(candidateY, pane); ok {
				return candidate, true
			}
			if p.split {
				if candidate, ok := p.cursorAt(candidateY, pane.Other()); ok {
					return candidate, true
				}
			}
		}
	}
	return Cursor{}, false
}

func (p *presentation) JumpFile(cursor Cursor, direction Direction) (Cursor, bool) {
	if !p.valid(cursor) {
		return Cursor{}, false
	}
	file, _ := p.file(cursor)
	y := cursor.Coordinate.Y
	for {
		next, ok := p.scan(y, cursor.Pane, direction, false)
		if !ok {
			return Cursor{}, false
		}
		nextFile, _ := p.file(next)
		if nextFile.Key() != file.Key() {
			return next, true
		}
		y = next.Coordinate.Y
	}
}

func (p *presentation) SwitchPane(cursor Cursor, pane Pane) (Cursor, bool) {
	if !p.split || !p.valid(cursor) {
		return Cursor{}, false
	}
	if candidate, ok := p.cursorAt(cursor.Coordinate.Y, pane); ok {
		return candidate, true
	}
	fileIndex := p.rows[cursor.Coordinate.Y].fileIndex
	for y := cursor.Coordinate.Y - 1; y >= 0 && p.rows[y].fileIndex == fileIndex; y-- {
		if candidate, ok := p.cursorAt(y, pane); ok {
			return candidate, true
		}
	}
	for y := cursor.Coordinate.Y + 1; y < len(p.rows) && p.rows[y].fileIndex == fileIndex; y++ {
		if candidate, ok := p.cursorAt(y, pane); ok {
			return candidate, true
		}
	}
	return Cursor{}, false
}

func (p *presentation) FindCursor(file diff.File, hunk diff.Hunk, line diff.Line, nearby Coordinate, pane Pane) (Cursor, bool) {
	var exact, sameText, sameKind, any []Cursor
	for y, current := range p.rows {
		if current.fileIndex < 0 || !diff.SameFile(p.changes.Files[current.fileIndex], file) || current.hunkIndex < 0 || current.hunkHeader != hunk.Header {
			continue
		}
		for _, candidatePane := range []Pane{pane, pane.Other()} {
			candidate, ok := p.cursorAt(y, candidatePane)
			if !ok {
				continue
			}
			candidateLine, _ := p.line(candidate)
			if candidateLine == line {
				exact = append(exact, candidate)
				continue
			}
			if candidateLine.Kind == line.Kind && candidateLine.Text == line.Text {
				sameText = append(sameText, candidate)
			}
			if candidateLine.Kind == line.Kind {
				sameKind = append(sameKind, candidate)
			}
			any = append(any, candidate)
		}
	}
	for _, candidates := range [][]Cursor{exact, sameText, sameKind, any} {
		if len(candidates) > 0 {
			return closest(candidates, nearby), true
		}
	}
	return Cursor{}, false
}

func closest(candidates []Cursor, nearby Coordinate) Cursor {
	best := candidates[0]
	for _, candidate := range candidates[1:] {
		if abs(candidate.Coordinate.Y-nearby.Y) < abs(best.Coordinate.Y-nearby.Y) {
			best = candidate
		}
	}
	return best
}

func abs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func highlightedLine(lines []string, number diff.LineNumber, fallback string) string {
	if number <= 0 || int(number) > len(lines) {
		return fallback
	}
	return lines[int(number)-1]
}
