package diffscreen

import (
	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

func beginSelection(cursor diffCursor) diffSelection {
	return diffSelection{First: cursor, Last: cursor}
}

func (v *diffView) extendSelectionToCursor(
	selection diffSelection,
	cursor diffCursor,
) (updated diffSelection, allowed bool) {
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

func (v *diffView) linesCoveredBySelection(selection diffSelection) []patch.Line {
	if _, ok := v.extendSelectionToCursor(selection, selection.Last); !ok {
		return nil
	}
	first, last := min(
		selection.First.row,
		selection.Last.row,
	), max(selection.First.row, selection.Last.row)
	if first == last && selection.First.pane != selection.Last.pane {
		return v.linesSelectedAcrossPanes(selection)
	}
	lines := make([]patch.Line, 0, last-first+1)
	for y := first; y <= last; y++ {
		pane := selection.paneForSelectedRow(y)
		if line, ok := v.line(diffCursor{row: y, pane: pane}); ok {
			lines = append(lines, line)
		}
	}
	return lines
}

func (s diffSelection) paneForSelectedRow(row int) diffPane {
	if row == s.Last.row {
		return s.Last.pane
	}
	return s.First.pane
}

func (v *diffView) linesSelectedAcrossPanes(selection diffSelection) []patch.Line {
	first, _ := v.line(selection.First)
	last, _ := v.line(selection.Last)
	if first == last {
		return []patch.Line{first}
	}
	return []patch.Line{first, last}
}

func (v *diffView) file(cursor diffCursor) (file patch.File, found bool) {
	if !v.valid(cursor) {
		return patch.File{}, false
	}
	return v.patch.Files[v.rows[cursor.row].file], true
}

func (v *diffView) line(cursor diffCursor) (line patch.Line, found bool) {
	if !v.valid(cursor) {
		return patch.Line{}, false
	}
	current := v.rows[cursor.row]
	return v.patch.Files[current.file].Hunks[current.hunk].Lines[v.sourceLineIndex(
		current,
		cursor.pane,
	)], true
}

// findCursor searches the file by line numbers, then matching text and kind,
// then source proximity. Ties keep the preferred pane.
func (v *diffView) findCursor(target cursorIdentity) (diffCursor, bool) {
	if !target.valid {
		return diffCursor{}, false
	}
	best := cursorMatch{}
	for y := range v.rows {
		candidate := v.matchRow(y, target)
		if candidate.rank == 3 {
			return candidate.cursor, true
		}
		if candidate.betterThan(best) {
			best = candidate
		}
	}
	return best.cursor, best.rank > 0
}

type cursorMatch struct {
	cursor         diffCursor
	rank, distance int
}

func (m cursorMatch) betterThan(other cursorMatch) bool {
	return m.rank > other.rank ||
		m.rank == other.rank && m.distance < other.distance
}

func (v *diffView) matchRow(y int, target cursorIdentity) cursorMatch {
	current := v.rows[y]
	if !v.belongsToFile(current, target.file) {
		return cursorMatch{}
	}
	best := cursorMatch{}
	for _, pane := range [2]diffPane{
		target.cursor.pane,
		target.cursor.pane.otherPane(),
	} {
		candidate, ok := v.cursorAt(y, pane)
		if !ok {
			continue
		}
		line, _ := v.line(candidate)
		match := cursorMatch{
			cursor:   candidate,
			rank:     lineMatchRank(line, target.line),
			distance: distanceFromTargetLine(line, target.line, y-target.cursor.row),
		}
		if match.rank == 3 {
			return match
		}
		if match.betterThan(best) {
			best = match
		}
	}
	return best
}

func (v *diffView) belongsToFile(current entry, file patch.File) bool {
	return current.hunk >= 0 && sameFile(v.patch.Files[current.file], file)
}

func lineMatchRank(candidateLine, targetLine patch.Line) (rank int) {
	if candidateLine.Kind != targetLine.Kind {
		return 1
	}
	if candidateLine.OldNumber == targetLine.OldNumber &&
		candidateLine.NewNumber == targetLine.NewNumber {
		return 3
	}
	if candidateLine.Text == targetLine.Text {
		return 2
	}
	return 1
}

func distanceFromTargetLine(candidateLine, targetLine patch.Line, rowDistance int) (distance int) {
	if candidateLine.NewNumber > 0 && targetLine.NewNumber > 0 {
		return abs(int(candidateLine.NewNumber - targetLine.NewNumber))
	}
	if candidateLine.OldNumber > 0 && targetLine.OldNumber > 0 {
		return abs(int(candidateLine.OldNumber - targetLine.OldNumber))
	}
	return abs(rowDistance)
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

func sourceLineOrFallback(
	lines []string,
	sourceLineNumber patch.LineNumber,
	fallback string,
) string {
	if sourceLineNumber <= 0 || int(sourceLineNumber) > len(lines) {
		return fallback
	}
	return lines[int(sourceLineNumber)-1]
}
