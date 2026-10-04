package ui

import (
	"fmt"
	"github.com/eskelinenantti/review-my-slop/internal/comments"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

func (v *diffView) beginSelection(cursor diffCursor) diffSelection {
	return diffSelection{First: cursor, Last: cursor}
}

func (v *diffView) extendSelection(selection diffSelection, cursor diffCursor) (diffSelection, bool) {
	if !v.valid(selection.First) || !v.valid(cursor) {
		return selection, false
	}
	first := v.rows[selection.First.coordinate.Y]
	last := v.rows[cursor.coordinate.Y]
	if first.file != last.file || first.hunk != last.hunk {
		return selection, false
	}
	selection.Last = cursor
	return selection, true
}

func (v *diffView) lines(selection diffSelection) []patch.Line {
	if _, ok := v.extendSelection(selection, selection.Last); !ok {
		return nil
	}
	first, last := selection.First.coordinate.Y, selection.Last.coordinate.Y
	if first > last {
		first, last = last, first
	}
	lines := make([]patch.Line, 0, last-first+1)
	if first == last && selection.First.pane != selection.Last.pane {
		current := v.rows[first]
		indices := []int{v.lineIndex(current, selection.First.pane), v.lineIndex(current, selection.Last.pane)}
		for _, index := range indices {
			if index >= 0 && (len(lines) == 0 || lines[len(lines)-1] != v.patch.Files[current.file].Hunks[current.hunk].Lines[index]) {
				lines = append(lines, v.patch.Files[current.file].Hunks[current.hunk].Lines[index])
			}
		}
		return lines
	}
	for y := first; y <= last; y++ {
		pane := selection.First.pane
		if y == selection.Last.coordinate.Y {
			pane = selection.Last.pane
		}
		if line, ok := v.line(diffCursor{coordinate: coordinate{Y: y}, pane: pane}); ok {
			lines = append(lines, line)
		}
	}
	return lines
}

func (v *diffView) anchor(selection diffSelection) (comments.Anchor, error) {
	lines := v.lines(selection)
	if len(lines) == 0 {
		return comments.Anchor{}, fmt.Errorf("select code lines before commenting")
	}
	first := v.rows[selection.First.coordinate.Y]
	file := v.patch.Files[first.file]
	hunk := file.Hunks[first.hunk]
	path := file.NewPath
	if path == "" {
		path = file.OldPath
	}
	anchor := comments.Anchor{FilePath: path}
	start, end := selection.First.coordinate.Y, selection.Last.coordinate.Y
	if start > end {
		start, end = end, start
	}
	for y := start; y <= end; y++ {
		panes := []diffPane{selection.First.pane}
		if start == end && selection.First.pane != selection.Last.pane {
			panes = append(panes, selection.Last.pane)
		} else if y == selection.Last.coordinate.Y {
			panes[0] = selection.Last.pane
		}
		for _, pane := range panes {
			index := v.lineIndex(v.rows[y], pane)
			if index < 0 {
				continue
			}
			line := hunk.Lines[index]
			prefix := " "
			if line.Kind == patch.Addition {
				prefix = "+"
			}
			if line.Kind == patch.Deletion {
				prefix = "-"
			}
			anchor.QuotedLines = append(anchor.QuotedLines, prefix+line.Text)
			accumulateRange(&anchor.OldStart, &anchor.OldEnd, int(line.OldNumber))
			accumulateRange(&anchor.NewStart, &anchor.NewEnd, int(line.NewNumber))
		}
	}
	return anchor, nil
}

func (v *diffView) file(cursor diffCursor) (patch.File, bool) {
	if !v.valid(cursor) {
		return patch.File{}, false
	}
	return v.patch.Files[v.rows[cursor.coordinate.Y].file], true
}

func (v *diffView) hunk(cursor diffCursor) (patch.Hunk, bool) {
	if !v.valid(cursor) {
		return patch.Hunk{}, false
	}
	current := v.rows[cursor.coordinate.Y]
	return v.patch.Files[current.file].Hunks[current.hunk], true
}

func (v *diffView) line(cursor diffCursor) (patch.Line, bool) {
	if !v.valid(cursor) {
		return patch.Line{}, false
	}
	current := v.rows[cursor.coordinate.Y]
	return v.patch.Files[current.file].Hunks[current.hunk].Lines[v.lineIndex(current, cursor.pane)], true
}

func (v *diffView) findCursor(file patch.File, hunk patch.Hunk, line patch.Line, nearby coordinate, pane diffPane) (diffCursor, bool) {
	candidates := make([]diffCursor, 0)
	fallbacks := make([]diffCursor, 0)
	nearbyCandidates := make([]diffCursor, 0)
	for y, current := range v.rows {
		if current.file < 0 || !sameFile(v.patch.Files[current.file], file) || current.hunk < 0 || v.patch.Files[current.file].Hunks[current.hunk].Header != hunk.Header {
			continue
		}
		for _, candidatePane := range []diffPane{pane, pane.other()} {
			candidate, ok := v.cursorAt(y, candidatePane)
			if !ok {
				continue
			}
			candidateLine, _ := v.line(candidate)
			if candidateLine.Kind == line.Kind && candidateLine.OldNumber == line.OldNumber && candidateLine.NewNumber == line.NewNumber {
				return candidate, true
			}
			if candidateLine.Kind == line.Kind && candidateLine.Text == line.Text {
				candidates = append(candidates, candidate)
			}
			if candidateLine.Kind == line.Kind {
				fallbacks = append(fallbacks, candidate)
			}
			nearbyCandidates = append(nearbyCandidates, candidate)
		}
	}
	if len(candidates) > 0 {
		return closest(candidates, nearby), true
	}
	if len(fallbacks) > 0 {
		return closest(fallbacks, nearby), true
	}
	if len(nearbyCandidates) > 0 {
		return closest(nearbyCandidates, nearby), true
	}
	return diffCursor{}, false
}

func sameFile(candidate, target patch.File) bool {
	return candidate.OldPath != "" && candidate.OldPath == target.OldPath ||
		candidate.NewPath != "" && candidate.NewPath == target.NewPath
}

func closest(candidates []diffCursor, nearby coordinate) diffCursor {
	best := candidates[0]
	for _, candidate := range candidates[1:] {
		if abs(candidate.coordinate.Y-nearby.Y) < abs(best.coordinate.Y-nearby.Y) {
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
