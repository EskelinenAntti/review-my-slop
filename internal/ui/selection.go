package ui

import (
	"fmt"

	"github.com/eskelinenantti/review-my-slop/internal/comment"
	"github.com/eskelinenantti/review-my-slop/internal/diff"
)

func (v *diffView) BeginSelection(cursor Cursor) Selection {
	return Selection{First: cursor, Last: cursor}
}

func (v *diffView) ExtendSelection(selection Selection, cursor Cursor) (Selection, bool) {
	if !v.valid(selection.First) || !v.valid(cursor) {
		return selection, false
	}
	first := v.rows[selection.First.Coordinate.Y]
	last := v.rows[cursor.Coordinate.Y]
	if first.file != last.file || first.hunk != last.hunk {
		return selection, false
	}
	selection.Last = cursor
	return selection, true
}

func (v *diffView) Lines(selection Selection) []diff.Line {
	if _, ok := v.ExtendSelection(selection, selection.Last); !ok {
		return nil
	}
	first, last := selection.First.Coordinate.Y, selection.Last.Coordinate.Y
	if first > last {
		first, last = last, first
	}
	lines := make([]diff.Line, 0, last-first+1)
	if first == last && selection.First.Pane != selection.Last.Pane {
		current := v.rows[first]
		indices := []int{v.lineIndex(current, selection.First.Pane), v.lineIndex(current, selection.Last.Pane)}
		for _, index := range indices {
			if index >= 0 && (len(lines) == 0 || lines[len(lines)-1] != v.changes.Files[current.file].Hunks[current.hunk].Lines[index]) {
				lines = append(lines, v.changes.Files[current.file].Hunks[current.hunk].Lines[index])
			}
		}
		return lines
	}
	for y := first; y <= last; y++ {
		pane := selection.First.Pane
		if y == selection.Last.Coordinate.Y {
			pane = selection.Last.Pane
		}
		if line, ok := v.Line(Cursor{Coordinate: Coordinate{Y: y}, Pane: pane}); ok {
			lines = append(lines, line)
		}
	}
	return lines
}

func (v *diffView) Anchor(selection Selection) (comment.Anchor, error) {
	lines := v.Lines(selection)
	if len(lines) == 0 {
		return comment.Anchor{}, fmt.Errorf("select code lines before commenting")
	}
	first := v.rows[selection.First.Coordinate.Y]
	file := v.changes.Files[first.file]
	hunk := file.Hunks[first.hunk]
	anchor := comment.Anchor{FilePath: file.Path()}
	start, end := selection.First.Coordinate.Y, selection.Last.Coordinate.Y
	if start > end {
		start, end = end, start
	}
	for y := start; y <= end; y++ {
		panes := []Pane{selection.First.Pane}
		if start == end && selection.First.Pane != selection.Last.Pane {
			panes = append(panes, selection.Last.Pane)
		} else if y == selection.Last.Coordinate.Y {
			panes[0] = selection.Last.Pane
		}
		for _, pane := range panes {
			index := v.lineIndex(v.rows[y], pane)
			if index < 0 {
				continue
			}
			line := hunk.Lines[index]
			prefix := " "
			if line.Kind == diff.Addition {
				prefix = "+"
			}
			if line.Kind == diff.Deletion {
				prefix = "-"
			}
			anchor.QuotedLines = append(anchor.QuotedLines, prefix+line.Text)
			addRange(&anchor.OldStart, &anchor.OldEnd, int(line.OldNumber))
			addRange(&anchor.NewStart, &anchor.NewEnd, int(line.NewNumber))
		}
	}
	return anchor, nil
}

func (v *diffView) File(cursor Cursor) (diff.File, bool) {
	if !v.valid(cursor) {
		return diff.File{}, false
	}
	return v.changes.Files[v.rows[cursor.Coordinate.Y].file], true
}

func (v *diffView) Hunk(cursor Cursor) (diff.Hunk, bool) {
	if !v.valid(cursor) {
		return diff.Hunk{}, false
	}
	current := v.rows[cursor.Coordinate.Y]
	return v.changes.Files[current.file].Hunks[current.hunk], true
}

func (v *diffView) Line(cursor Cursor) (diff.Line, bool) {
	if !v.valid(cursor) {
		return diff.Line{}, false
	}
	current := v.rows[cursor.Coordinate.Y]
	return v.changes.Files[current.file].Hunks[current.hunk].Lines[v.lineIndex(current, cursor.Pane)], true
}

func (v *diffView) FindCursor(file diff.File, hunk diff.Hunk, line diff.Line, nearby Coordinate, pane Pane) (Cursor, bool) {
	var exactText, sameKind, nearbyLines []Cursor
	for y, current := range v.rows {
		if current.file < 0 || !sameFile(v.changes.Files[current.file], file) || current.hunk < 0 || v.changes.Files[current.file].Hunks[current.hunk].Header != hunk.Header {
			continue
		}
		for _, candidatePane := range []Pane{pane, pane.Other()} {
			candidate, ok := v.cursorAt(y, candidatePane)
			if !ok {
				continue
			}
			candidateLine, _ := v.Line(candidate)
			if candidateLine.Kind == line.Kind && candidateLine.OldNumber == line.OldNumber && candidateLine.NewNumber == line.NewNumber {
				return candidate, true
			}
			if candidateLine.Kind == line.Kind && candidateLine.Text == line.Text {
				exactText = append(exactText, candidate)
			}
			if candidateLine.Kind == line.Kind {
				sameKind = append(sameKind, candidate)
			}
			nearbyLines = append(nearbyLines, candidate)
		}
	}
	for _, candidates := range [][]Cursor{exactText, sameKind, nearbyLines} {
		if len(candidates) > 0 {
			return closest(candidates, nearby), true
		}
	}
	return Cursor{}, false
}

func sameFile(candidate, target diff.File) bool {
	return candidate.OldPath != "" && candidate.OldPath == target.OldPath || candidate.NewPath != "" && candidate.NewPath == target.NewPath
}

func closest(candidates []Cursor, nearby Coordinate) Cursor {
	best := candidates[0]
	for _, candidate := range candidates[1:] {
		if absolute(candidate.Coordinate.Y-nearby.Y) < absolute(best.Coordinate.Y-nearby.Y) {
			best = candidate
		}
	}
	return best
}

func addRange(start, end *int, value int) {
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

func absolute(value int) int {
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
