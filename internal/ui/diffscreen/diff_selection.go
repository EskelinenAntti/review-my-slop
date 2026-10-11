package diffscreen

import (
	"iter"

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
	if selection.First.row == selection.Last.row && selection.First.pane != selection.Last.pane {
		return v.crossPaneLines(selection)
	}
	return v.paneLines(selection)
}

func (v *diffView) crossPaneLines(selection diffSelection) []patch.Line {
	var lines []patch.Line
	for _, pane := range []diffPane{selection.First.pane, selection.Last.pane} {
		lines = v.appendDistinctLine(lines, diffCursor{row: selection.First.row, pane: pane})
	}
	return lines
}

func (v *diffView) paneLines(selection diffSelection) []patch.Line {
	var lines []patch.Line
	first, last := min(selection.First.row, selection.Last.row), max(selection.First.row, selection.Last.row)
	for y := first; y <= last; y++ {
		if line, ok := v.line(diffCursor{row: y, pane: selectionPane(selection, y)}); ok {
			lines = append(lines, line)
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

func (v *diffView) appendDistinctLine(lines []patch.Line, cursor diffCursor) []patch.Line {
	line, ok := v.line(cursor)
	if !ok {
		return lines
	}
	if len(lines) == 0 || lines[len(lines)-1] != line {
		return append(lines, line)
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
	for candidate := range v.fileCursors(target) {
		match := v.rankCursor(candidate, target)
		if match.rank == 3 {
			return candidate, true
		}
		best = betterMatch(best, match)
	}
	return best.cursor, best.rank > 0
}

type cursorMatch struct {
	cursor         diffCursor
	rank, distance int
}

func (v *diffView) fileCursors(target cursorIdentity) iter.Seq[diffCursor] {
	return func(yield func(diffCursor) bool) {
		v.yieldFileCursors(target, yield)
	}
}

func (v *diffView) yieldFileCursors(target cursorIdentity, yield func(diffCursor) bool) {
	for y, current := range v.rows {
		if !v.visitFileRow(y, current, target, yield) {
			return
		}
	}
}

func (v *diffView) visitFileRow(y int, current entry, target cursorIdentity, yield func(diffCursor) bool) bool {
	if !v.isTargetFile(current, target.file) {
		return true
	}
	return v.yieldRowCursors(y, target.cursor.pane, yield)
}

func (v *diffView) isTargetFile(current entry, file patch.File) bool {
	return current.hunk >= 0 && sameFile(v.patch.Files[current.file], file)
}

func (v *diffView) yieldRowCursors(y int, pane diffPane, yield func(diffCursor) bool) bool {
	for _, pane := range [2]diffPane{pane, pane.other()} {
		if candidate, ok := v.cursorAt(y, pane); ok && !yield(candidate) {
			return false
		}
	}
	return true
}

func (v *diffView) rankCursor(candidate diffCursor, target cursorIdentity) cursorMatch {
	line, _ := v.line(candidate)
	return cursorMatch{cursor: candidate, rank: identityRank(line, target.line), distance: sourceDistance(line, candidate.row, target)}
}

func identityRank(line, target patch.Line) int {
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

func sourceDistance(line patch.Line, row int, target cursorIdentity) int {
	if line.NewNumber > 0 && target.line.NewNumber > 0 {
		return abs(int(line.NewNumber - target.line.NewNumber))
	}
	if line.OldNumber > 0 && target.line.OldNumber > 0 {
		return abs(int(line.OldNumber - target.line.OldNumber))
	}
	return abs(row - target.cursor.row)
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
