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
	if !v.sameFileHunk(selection.First, cursor) {
		return selection, false
	}
	selection.Last = cursor
	return selection, true
}

func (v *diffView) sameFileHunk(first, last diffCursor) bool {
	if !v.valid(first) || !v.valid(last) {
		return false
	}
	firstRow, lastRow := v.rows[first.row], v.rows[last.row]
	return firstRow.file == lastRow.file && firstRow.hunk == lastRow.hunk
}

func (v *diffView) lines(selection diffSelection) []patch.Line {
	if !v.sameFileHunk(selection.First, selection.Last) {
		return nil
	}
	first, last := min(
		selection.First.row,
		selection.Last.row,
	), max(selection.First.row, selection.Last.row)
	if first == last && selection.First.pane != selection.Last.pane {
		return v.paneLines(selection)
	}
	lines := make([]patch.Line, 0, last-first+1)
	for y := first; y <= last; y++ {
		pane := selection.paneAt(y)
		if line, ok := v.line(diffCursor{row: y, pane: pane}); ok {
			lines = append(lines, line)
		}
	}
	return lines
}

func (s diffSelection) paneAt(row int) diffPane {
	if row == s.Last.row {
		return s.Last.pane
	}
	return s.First.pane
}

func (v *diffView) paneLines(selection diffSelection) []patch.Line {
	first, _ := v.line(selection.First)
	last, _ := v.line(selection.Last)
	firstIndex := v.lineIndex(v.rows[selection.First.row], selection.First.pane)
	lastIndex := v.lineIndex(v.rows[selection.Last.row], selection.Last.pane)
	if firstIndex == lastIndex {
		return []patch.Line{first}
	}
	return []patch.Line{first, last}
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
	return v.patch.Files[current.file].Hunks[current.hunk].Lines[v.lineIndex(
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
		best.consider(candidate)
		if candidate.rank == cursorMatchByLineNumbers {
			return candidate.cursor, true
		}
	}
	return best.cursor, best.rank > noCursorMatch
}

type cursorMatch struct {
	cursor         diffCursor
	rank, distance int
}

const (
	noCursorMatch = iota
	cursorMatchInFile
	cursorMatchByText
	cursorMatchByLineNumbers
)

func (m cursorMatch) betterThan(other cursorMatch) bool {
	return m.rank > other.rank ||
		m.rank == other.rank && m.distance < other.distance
}

func (m *cursorMatch) consider(candidate cursorMatch) {
	if candidate.betterThan(*m) {
		*m = candidate
	}
}

func (v *diffView) matchRow(y int, target cursorIdentity) cursorMatch {
	current := v.rows[y]
	if !v.rowInFile(current, target) {
		return cursorMatch{}
	}
	best := cursorMatch{}
	for _, pane := range [2]diffPane{
		target.cursor.pane,
		target.cursor.pane.other(),
	} {
		candidate, ok := v.cursorAt(y, pane)
		if !ok {
			continue
		}
		line, _ := v.line(candidate)
		match := cursorMatch{
			cursor:   candidate,
			rank:     lineMatchRank(line, target.line),
			distance: lineDistance(line, target.line, y-target.cursor.row),
		}
		best.consider(match)
		if match.rank == cursorMatchByLineNumbers {
			return match
		}
	}
	return best
}

func (v *diffView) rowInFile(current entry, target cursorIdentity) bool {
	if current.hunk < 0 {
		return false
	}
	file := v.patch.Files[current.file]
	return sameFilePaths(
		file.OldPath,
		file.NewPath,
		target.oldPath,
		target.newPath,
	)
}

func lineMatchRank(line, target patch.Line) int {
	if line.Kind != target.Kind {
		return cursorMatchInFile
	}
	if line.OldNumber == target.OldNumber &&
		line.NewNumber == target.NewNumber {
		return cursorMatchByLineNumbers
	}
	if line.Text == target.Text {
		return cursorMatchByText
	}
	return cursorMatchInFile
}

func lineDistance(line, target patch.Line, rowDistance int) int {
	distance, ok := lineNumberDistance(line.NewNumber, target.NewNumber)
	if ok {
		return distance
	}
	distance, ok = lineNumberDistance(line.OldNumber, target.OldNumber)
	if ok {
		return distance
	}
	return abs(rowDistance)
}

func lineNumberDistance(first, second patch.LineNumber) (int, bool) {
	if first <= 0 || second <= 0 {
		return 0, false
	}
	return abs(int(first - second)), true
}

func sameFilePaths(
	candidateOld, candidateNew, targetOld, targetNew string,
) bool {
	oldPathMatches := candidateOld != "" && candidateOld == targetOld
	newPathMatches := candidateNew != "" && candidateNew == targetNew
	return oldPathMatches || newPathMatches
}

func abs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func highlightedLine(
	lines []string,
	number patch.LineNumber,
	fallback string,
) string {
	if number <= 0 || int(number) > len(lines) {
		return fallback
	}
	return lines[int(number)-1]
}
