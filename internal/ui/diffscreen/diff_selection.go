package diffscreen

import (
	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

func (v *diffView) beginSelection(cursor diffCursor) diffSelection {
	return diffSelection{First: cursor, Last: cursor}
}

func (v *diffView) extendSelection(selection diffSelection, cursor diffCursor) (diffSelection, bool) {
	if !v.valid(selection.First) || !v.valid(cursor) {
		return selection, false
	}
	first := v.rows[selection.First.row]
	last := v.rows[cursor.row]
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
	first, last := selection.First.row, selection.Last.row
	first, last = min(first, last), max(first, last)
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
		if y == selection.Last.row {
			pane = selection.Last.pane
		}
		if line, ok := v.line(diffCursor{row: y, pane: pane}); ok {
			lines = append(lines, line)
		}
	}
	return lines
}

func (v *diffView) file(cursor diffCursor) (patch.File, bool) {
	if !v.valid(cursor) {
		return patch.File{}, false
	}
	return v.patch.Files[v.rows[cursor.row].file], true
}

func (v *diffView) line(cursor diffCursor) (patch.Line, bool) {
	if !v.valid(cursor) {
		return patch.Line{}, false
	}
	current := v.rows[cursor.row]
	return v.patch.Files[current.file].Hunks[current.hunk].Lines[v.lineIndex(current, cursor.pane)], true
}

// findCursor prioritizes line numbers, then matching text and kind, then kind,
// then any line in the same hunk. Ties keep the closest row and preferred pane.
func (v *diffView) findCursor(target cursorIdentity) (diffCursor, bool) {
	if !target.valid {
		return diffCursor{}, false
	}
	best := diffCursor{}
	bestRank, bestDistance := 0, 0
	for y, current := range v.rows {
		if current.hunk < 0 || !sameFile(v.patch.Files[current.file], target.file) || v.patch.Files[current.file].Hunks[current.hunk].Header != target.hunk.Header {
			continue
		}
		for _, pane := range [2]diffPane{target.cursor.pane, target.cursor.pane.other()} {
			candidate, ok := v.cursorAt(y, pane)
			if !ok {
				continue
			}
			line, _ := v.line(candidate)
			rank := 1
			if line.Kind == target.line.Kind {
				if line.OldNumber == target.line.OldNumber && line.NewNumber == target.line.NewNumber {
					return candidate, true
				}
				rank = 2
				if line.Text == target.line.Text {
					rank = 3
				}
			}
			distance := abs(y - target.cursor.row)
			if rank > bestRank || rank == bestRank && distance < bestDistance {
				best, bestRank, bestDistance = candidate, rank, distance
			}
		}
	}
	return best, bestRank > 0
}

func sameFile(candidate, target patch.File) bool {
	return candidate.OldPath != "" && candidate.OldPath == target.OldPath ||
		candidate.NewPath != "" && candidate.NewPath == target.NewPath
}

func abs(value int) int { return max(value, -value) }

func highlightedLine(lines []string, number patch.LineNumber, fallback string) string {
	if number <= 0 || int(number) > len(lines) {
		return fallback
	}
	return lines[int(number)-1]
}
