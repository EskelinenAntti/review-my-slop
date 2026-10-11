package diffscreen

import (
	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"iter"
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
	first, last := min(selection.First.row, selection.Last.row), max(selection.First.row, selection.Last.row)
	if crossPaneSelection(selection) {
		return v.crossPaneLines(selection)
	}
	return v.rangeLines(selection, first, last)
}

func (v *diffView) rangeLines(selection diffSelection, first, last int) []patch.Line {
	lines := make([]patch.Line, 0, last-first+1)
	for y := first; y <= last; y++ {
		pane := selectionPane(selection, y)
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

// findCursor searches the file by line numbers, then matching text and kind,
// then source proximity. Ties keep the preferred pane.
func (v *diffView) findCursor(target cursorIdentity) (diffCursor, bool) {
	if !target.valid {
		return diffCursor{}, false
	}
	best := cursorMatch{}
	for y := range v.matchingRows(target.file) {
		match := v.matchRow(y, target)
		if match.rank == 3 {
			return match.cursor, true
		}
		best = betterMatch(best, match)
	}
	return best.cursor, best.rank > 0
}

type cursorMatch struct {
	cursor         diffCursor
	rank, distance int
}

func (v *diffView) matchesFile(row entry, file patch.File) bool {
	return row.hunk >= 0 && sameFile(v.patch.Files[row.file], file)
}

func (v *diffView) matchRow(y int, target cursorIdentity) cursorMatch {
	best := cursorMatch{}
	for _, pane := range [2]diffPane{target.cursor.pane, target.cursor.pane.other()} {
		candidate, ok := v.cursorAt(y, pane)
		if !ok {
			continue
		}
		line, _ := v.line(candidate)
		match := cursorMatch{cursor: candidate, rank: lineRank(line, target.line), distance: lineDistance(line, target.line, y-target.cursor.row)}
		if match.rank == 3 {
			return match
		}
		best = betterMatch(best, match)
	}
	return best
}

func lineRank(line, target patch.Line) int {
	if line.Kind != target.Kind {
		return 1
	}
	if line.OldNumber == target.OldNumber && line.NewNumber == target.NewNumber {
		return 3
	}
	if line.Text == target.Text {
		return 2
	}
	return 1
}

func lineDistance(line, target patch.Line, rows int) int {
	if line.NewNumber > 0 && target.NewNumber > 0 {
		return abs(int(line.NewNumber - target.NewNumber))
	}
	if line.OldNumber > 0 && target.OldNumber > 0 {
		return abs(int(line.OldNumber - target.OldNumber))
	}
	return abs(rows)
}

func betterMatch(best, candidate cursorMatch) cursorMatch {
	if candidate.rank > best.rank || candidate.rank == best.rank && candidate.distance < best.distance {
		return candidate
	}
	return best
}

func sameFile(candidate, target patch.File) bool {
	return candidate.OldPath != "" && candidate.OldPath == target.OldPath ||
		candidate.NewPath != "" && candidate.NewPath == target.NewPath
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

func selectionPane(selection diffSelection, y int) diffPane {
	if y == selection.Last.row {
		return selection.Last.pane
	}
	return selection.First.pane
}

func (v *diffView) crossPaneLines(selection diffSelection) []patch.Line {
	var lines []patch.Line
	for _, pane := range []diffPane{selection.First.pane, selection.Last.pane} {
		line, ok := v.line(diffCursor{row: selection.First.row, pane: pane})
		if ok && !containsLine(lines, line) {
			lines = append(lines, line)
		}
	}
	return lines
}
func containsLine(lines []patch.Line, line patch.Line) bool {
	for _, existing := range lines {
		if existing == line {
			return true
		}
	}
	return false
}

func (v *diffView) matchingRows(file patch.File) iter.Seq[int] {
	return func(yield func(int) bool) {
		for y, current := range v.rows {
			if v.matchesFile(current, file) && !yield(y) {
				return
			}
		}
	}
}
