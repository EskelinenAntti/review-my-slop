package ui

import (
	"fmt"
	"github.com/eskelinenantti/review-my-slop/internal/comments"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

func (v *diffView) ExtendSelection(selection Selection, cursor Cursor) (Selection, bool) {
	if !v.valid(selection.First) || !v.valid(cursor) {
		return selection, false
	}
	first := v.rows[selection.First.Coordinate]
	last := v.rows[cursor.Coordinate]
	if first.file != last.file || first.hunk != last.hunk {
		return selection, false
	}
	selection.Last = cursor
	return selection, true
}

func (v *diffView) Anchor(selection Selection) (comments.Anchor, error) {
	lines := v.selectedLines(selection, false)
	if len(lines) == 0 {
		return comments.Anchor{}, fmt.Errorf("select code lines before commenting")
	}
	file, _ := v.File(selection.First)
	filePath := file.NewPath
	if filePath == "" {
		filePath = file.OldPath
	}
	anchor := comments.Anchor{FilePath: filePath}
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
	firstCursor, lastCursor := selection.First, selection.Last
	if _, ok := v.ExtendSelection(selection, lastCursor); !ok {
		return nil
	}
	first, last := firstCursor.Coordinate, lastCursor.Coordinate
	if first > last {
		first, last = last, first
	}
	firstPane, lastPane := firstCursor.Pane, lastCursor.Pane
	var lines []patch.Line
	for y := first; y <= last; y++ {
		panes := []Pane{firstPane}
		if first == last && firstPane != lastPane {
			panes = append(panes, lastPane)
		} else if y == lastCursor.Coordinate {
			panes[0] = lastPane
		}
		for _, pane := range panes {
			line, ok := v.Line(Cursor{y, pane})
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
	return v.patch.Files[v.rows[cursor.Coordinate].file], true
}

func (v *diffView) Line(cursor Cursor) (patch.Line, bool) {
	if !v.valid(cursor) {
		return patch.Line{}, false
	}
	current := v.rows[cursor.Coordinate]
	return v.patch.Files[current.file].Hunks[current.hunk].Lines[v.lineIndex(current, cursor.Pane)], true
}

func (v *diffView) FindCursor(file patch.File, hunk patch.Hunk, line patch.Line, nearby int, pane Pane) (Cursor, bool) {
	var matches [3][]Cursor
	for y, current := range v.rows {
		if current.file < 0 {
			continue
		}
		candidateFile := v.patch.Files[current.file]
		if !sameFile(candidateFile, file) || current.hunk < 0 || candidateFile.Hunks[current.hunk].Header != hunk.Header {
			continue
		}
		for _, candidatePane := range []Pane{pane, Right - pane} {
			candidate, ok := v.cursorAt(y, candidatePane)
			if !ok {
				continue
			}
			candidateLine, _ := v.Line(candidate)
			match := 0
			if candidateLine.Kind == line.Kind {
				if candidateLine.OldNumber == line.OldNumber && candidateLine.NewNumber == line.NewNumber {
					return candidate, true
				}
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
			best := matches[match][0]
			for _, candidate := range matches[match][1:] {
				distance, bestDistance := candidate.Coordinate-nearby, best.Coordinate-nearby
				if max(distance, -distance) < max(bestDistance, -bestDistance) {
					best = candidate
				}
			}
			return best, true
		}
	}
	return Cursor{}, false
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

func sameFile(candidate, target patch.File) bool {
	return candidate.OldPath != "" && candidate.OldPath == target.OldPath ||
		candidate.NewPath != "" && candidate.NewPath == target.NewPath
}

func highlightedLine(lines []string, number patch.LineNumber, fallback string) string {
	if number <= 0 || int(number) > len(lines) {
		return fallback
	}
	return lines[int(number)-1]
}
