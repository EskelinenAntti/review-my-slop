package diffscreen

import (
	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

func (*diffView) beginSelection(cursor diffCursor) diffSelection {
	return diffSelection{First: cursor, Last: cursor}
}

func (v *diffView) extendSelection(selection diffSelection, cursor diffCursor) (

	diffSelection,

	bool,
) {
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
	if first > last {
		first, last = last, first
	}
	lines := make([]patch.Line, zero, last-first+one)
	if first == last && selection.First.pane != selection.Last.pane {
		return v.pairedSelectionLines(first, selection)
	}
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
	for y, current := range v.rows {
		if current.hunk < zero ||
			!sameFile(v.patch.Files[current.file], target.file) {
			continue
		}
		match := v.matchRow(y, target)
		if match.exact {
			return match.cursor, true
		}
		best = best.choose(match)
	}
	return best.cursor, best.rank > zero
}

type cursorMatch struct {
	cursor         diffCursor
	rank, distance int
	exact          bool
}

func (m cursorMatch) betterThan(other cursorMatch) bool {
	return m.rank > other.rank ||
		m.rank == other.rank && m.distance < other.distance
}

func (v *diffView) matchRow(y int, target cursorIdentity) cursorMatch {
	best := cursorMatch{}
	for _, pane := range [two]diffPane{
		target.cursor.pane,
		target.cursor.pane.other(),
	} {
		candidate, ok := v.cursorAt(y, pane)
		if !ok {
			continue
		}
		line, _ := v.line(candidate)
		match := matchLine(candidate, line, target)
		if match.exact {
			return match
		}
		best = best.choose(match)
	}
	return best
}

func matchLine(
	cursor diffCursor,
	line patch.Line,
	target cursorIdentity,
) cursorMatch {
	match := cursorMatch{
		cursor:   cursor,
		rank:     one,
		distance: lineDistance(cursor, line, target),
	}
	if line.Kind != target.line.Kind {
		return match
	}
	if line.OldNumber == target.line.OldNumber &&
		line.NewNumber == target.line.NewNumber {
		match.exact = true
		return match
	}
	if line.Text == target.line.Text {
		match.rank = two
	}
	return match
}

func lineDistance(
	cursor diffCursor,
	line patch.Line,
	target cursorIdentity,
) int {
	if line.NewNumber > zero && target.line.NewNumber > zero {
		return abs(int(line.NewNumber - target.line.NewNumber))
	}
	if line.OldNumber > zero && target.line.OldNumber > zero {
		return abs(int(line.OldNumber - target.line.OldNumber))
	}
	return abs(cursor.row - target.cursor.row)
}

func sameFile(candidate, target patch.File) bool {
	return candidate.OldPath != "" && candidate.OldPath == target.OldPath ||
		candidate.NewPath != "" && candidate.NewPath == target.NewPath
}

func abs(value int) int {
	if value < zero {
		return -value
	}
	return value
}

func highlightedLine(
	lines []string,
	number patch.LineNumber,
	fallback string,
) string {
	if number <= zero || int(number) > len(lines) {
		return fallback
	}
	return lines[int(number)-one]
}

func (v *diffView) pairedSelectionLines(
	rowIndex int,
	selection diffSelection,
) []patch.Line {
	lines := make([]patch.Line, zero, two)
	current := v.rows[rowIndex]
	sourceLines := v.patch.Files[current.file].Hunks[current.hunk].Lines
	indices := []int{
		v.lineIndex(current, selection.First.pane),
		v.lineIndex(current, selection.Last.pane),
	}
	for _, index := range indices {
		if index >= zero &&

			(len(lines) == zero ||
				lines[len(lines)-one] != sourceLines[index]) {
			lines = append(lines, sourceLines[index])
		}
	}
	return lines
}

func selectionPane(selection diffSelection, row int) diffPane {
	if row == selection.Last.row {
		return selection.Last.pane
	}
	return selection.First.pane
}

func (m cursorMatch) choose(other cursorMatch) cursorMatch {
	if other.betterThan(m) {
		return other
	}
	return m
}
