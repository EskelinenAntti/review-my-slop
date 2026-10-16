package diffscreen

import (
	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

func beginSelection(cursor diffCursor) diffSelection {
	return diffSelection{First: cursor, Last: cursor}
}

func (v *diffView) extendSelection(
	selection diffSelection,
	cursor diffCursor,
) (diffSelection, bool) {
	if !v.isChangedLine(selection.First) || !v.isChangedLine(cursor) {
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

func (v *diffView) selectedLines(selection diffSelection) []patch.Line {
	if _, ok := v.extendSelection(selection, selection.Last); !ok {
		return nil
	}
	first, last := min(
		selection.First.row,
		selection.Last.row,
	), max(selection.First.row, selection.Last.row)
	if first == last && selection.First.pane != selection.Last.pane {
		return v.selectedLinesInPane(selection)
	}
	lines := make([]patch.Line, 0, last-first+1)
	for y := first; y <= last; y++ {
		pane := selection.paneForRow(y)
		line, ok := v.patchLineForPosition(
			diffCursor{row: y, pane: pane},
		)
		if ok {
			lines = append(lines, line)
		}
	}
	return lines
}

func (s diffSelection) paneForRow(row int) diffPane {
	if row == s.Last.row {
		return s.Last.pane
	}
	return s.First.pane
}

func (v *diffView) selectedLinesInPane(selection diffSelection) []patch.Line {
	first, _ := v.patchLineForPosition(selection.First)
	last, _ := v.patchLineForPosition(selection.Last)
	if first == last {
		return []patch.Line{first}
	}
	return []patch.Line{first, last}
}

func (v *diffView) fileForLine(cursor diffCursor) (patch.File, bool) {
	if !v.isChangedLine(cursor) {
		return patch.File{}, false
	}
	return v.patch.Files[v.rows[cursor.row].file], true
}

func (v *diffView) patchLineForPosition(cursor diffCursor) (patch.Line, bool) {
	if !v.isChangedLine(cursor) {
		return patch.Line{}, false
	}
	current := v.rows[cursor.row]
	linePosition := v.findLinePositionInChangeBlock(current, cursor.pane)
	line := v.patch.Files[current.file].Hunks[current.hunk].Lines[linePosition]
	return line, true
}

// findCursor searches the file by line numbers, then matching text and kind,
// then source proximity. Ties keep the preferred pane.
func (v *diffView) findSameLineAfterChanges(
	target focusedLineDetails,
) (diffCursor, bool) {
	if !target.valid {
		return diffCursor{}, false
	}
	best := cursorMatch{}
	for y := range v.rows {
		candidate := v.matchLineRow(y, target)
		if candidate.rank == 3 {
			return candidate.cursor, true
		}
		if candidate.outranks(best) {
			best = candidate
		}
	}
	return best.cursor, best.rank > 0
}

type cursorMatch struct {
	cursor         diffCursor
	rank, distance int
}

func (m cursorMatch) outranks(other cursorMatch) bool {
	return m.rank > other.rank ||
		m.rank == other.rank && m.distance < other.distance
}

func (m cursorMatch) chooseBetterPosition(other cursorMatch) cursorMatch {
	if m.outranks(other) {
		return m
	}
	return other
}

func (v *diffView) matchLineRow(y int, target focusedLineDetails) cursorMatch {
	current := v.rows[y]
	if !v.rowBelongsToFile(current, target.file) {
		return cursorMatch{}
	}
	best := cursorMatch{}
	for _, pane := range [2]diffPane{
		target.cursor.pane,
		target.cursor.pane.oppositePane(),
	} {
		candidate, ok := v.changedLineAtRow(y, pane)
		if !ok {
			continue
		}
		line, _ := v.patchLineForPosition(candidate)
		match := cursorMatch{
			cursor:   candidate,
			rank:     rankLineMatch(line, target.line),
			distance: distanceFromLine(line, target.line, y-target.cursor.row),
		}
		if match.rank == 3 {
			return match
		}
		best = match.chooseBetterPosition(best)
	}
	return best
}

func (v *diffView) rowBelongsToFile(current entry, file patch.File) bool {
	return current.hunk >= 0 && sameFile(v.patch.Files[current.file], file)
}

func rankLineMatch(line, target patch.Line) int {
	if line.Kind != target.Kind {
		return 1
	}
	if line.OldNumber == target.OldNumber &&
		line.NewNumber == target.NewNumber {
		return 3
	}
	if line.Text == target.Text {
		return 2
	}
	return 1
}

func distanceFromLine(line, target patch.Line, rowDistance int) int {
	if line.NewNumber > 0 && target.NewNumber > 0 {
		return absoluteValue(int(line.NewNumber - target.NewNumber))
	}
	if line.OldNumber > 0 && target.OldNumber > 0 {
		return absoluteValue(int(line.OldNumber - target.OldNumber))
	}
	return absoluteValue(rowDistance)
}

func sameFile(candidate, target patch.File) bool {
	return candidate.OldPath != "" && candidate.OldPath == target.OldPath ||
		candidate.NewPath != "" && candidate.NewPath == target.NewPath
}

func absoluteValue(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func lineWithHighlight(
	lines []string,
	number patch.LineNumber,
	fallback string,
) string {
	if number <= 0 || int(number) > len(lines) {
		return fallback
	}
	return lines[int(number)-1]
}
