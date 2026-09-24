package ui

import (
	"fmt"
	"github.com/eskelinenantti/review-my-slop/internal/comments"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
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

func (v *diffView) Lines(selection Selection) []patch.Line {
	first, last := selection.First.Coordinate.Y, selection.Last.Coordinate.Y
	return v.selectedLines(selection, first == last && selection.First.Pane != selection.Last.Pane)
}

func (v *diffView) Anchor(selection Selection) (comments.Anchor, error) {
	lines := v.selectedLines(selection, false)
	if len(lines) == 0 {
		return comments.Anchor{}, fmt.Errorf("select code lines before commenting")
	}
	file, _ := v.File(selection.First)
	anchor := comments.Anchor{FilePath: file.Path()}
	for _, line := range lines {
		prefix := " "
		switch line.Kind {
		case patch.Addition:
			prefix = "+"
		case patch.Deletion:
			prefix = "-"
		}
		anchor.QuotedLines = append(anchor.QuotedLines, prefix+line.Text)
		accumulateRange(&anchor.OldStart, &anchor.OldEnd, int(line.OldNumber))
		accumulateRange(&anchor.NewStart, &anchor.NewEnd, int(line.NewNumber))
	}
	return anchor, nil
}

func (v *diffView) selectedLines(selection Selection, deduplicate bool) []patch.Line {
	if _, ok := v.ExtendSelection(selection, selection.Last); !ok {
		return nil
	}
	first, last := selection.First.Coordinate.Y, selection.Last.Coordinate.Y
	if first > last {
		first, last = last, first
	}
	lines := make([]patch.Line, 0, last-first+1)
	for y := first; y <= last; y++ {
		panes := []Pane{selection.First.Pane}
		if first == last && selection.First.Pane != selection.Last.Pane {
			panes = append(panes, selection.Last.Pane)
		} else if y == selection.Last.Coordinate.Y {
			panes[0] = selection.Last.Pane
		}
		for _, pane := range panes {
			line, ok := v.Line(Cursor{Coordinate: Coordinate{Y: y}, Pane: pane})
			if !ok || deduplicate && len(lines) > 0 && lines[len(lines)-1] == line {
				continue
			}
			lines = append(lines, line)
		}
	}
	return lines
}

func (v *diffView) File(cursor Cursor) (patch.File, bool) {
	if !v.valid(cursor) {
		return patch.File{}, false
	}
	return v.patch.Files[v.rows[cursor.Coordinate.Y].file], true
}

func (v *diffView) Hunk(cursor Cursor) (patch.Hunk, bool) {
	if !v.valid(cursor) {
		return patch.Hunk{}, false
	}
	current := v.rows[cursor.Coordinate.Y]
	return v.patch.Files[current.file].Hunks[current.hunk], true
}

func (v *diffView) Line(cursor Cursor) (patch.Line, bool) {
	if !v.valid(cursor) {
		return patch.Line{}, false
	}
	current := v.rows[cursor.Coordinate.Y]
	return v.patch.Files[current.file].Hunks[current.hunk].Lines[v.lineIndex(current, cursor.Pane)], true
}

func (v *diffView) FindCursor(file patch.File, hunk patch.Hunk, line patch.Line, nearby Coordinate, pane Pane) (Cursor, bool) {
	matches := [3][]Cursor{}
	for y, current := range v.rows {
		if current.file < 0 || !sameFile(v.patch.Files[current.file], file) || current.hunk < 0 || v.patch.Files[current.file].Hunks[current.hunk].Header != hunk.Header {
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
			match := 0
			if candidateLine.Kind == line.Kind {
				match = 1
				if candidateLine.Text == line.Text {
					match = 2
				}
			}
			matches[match] = append(matches[match], candidate)
		}
	}
	for match := len(matches) - 1; match >= 0; match-- {
		if len(matches[match]) > 0 {
			return closest(matches[match], nearby), true
		}
	}
	return Cursor{}, false
}

func sameFile(candidate, target patch.File) bool {
	return candidate.OldPath != "" && candidate.OldPath == target.OldPath ||
		candidate.NewPath != "" && candidate.NewPath == target.NewPath
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

func abs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func highlightedLine(lines []string, number patch.LineNumber, fallback string) string {
	if number <= 0 || int(number) > len(lines) {
		return fallback
	}
	return lines[int(number)-1]
}
