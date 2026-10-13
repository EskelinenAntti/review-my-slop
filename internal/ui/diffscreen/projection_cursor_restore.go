package diffscreen

import "github.com/eskelinenantti/review-my-slop/internal/patch"

type cursorCandidate struct {
	cursor         patchCursor
	rank, distance int
}

// restoreCursor searches the original file, preferring exact line identity,
// then matching text and kind, then source proximity.
func (v *patchProjection) restoreCursor(
	target cursorIdentity,
) (patchCursor, bool) {
	if !target.valid {
		return patchCursor{}, false
	}
	best := cursorCandidate{}
	for row := range v.rows {
		candidate := v.matchCursorRow(row, target)
		if candidate.rank == 3 {
			return candidate.cursor, true
		}
		if candidate.betterThan(best) {
			best = candidate
		}
	}
	return best.cursor, best.rank > 0
}

// matchCursorRow finds the strongest candidate in one display row.
func (v *patchProjection) matchCursorRow(
	row int,
	target cursorIdentity,
) cursorCandidate {
	displayRow := v.rows[row]
	if !v.rowBelongsToFile(displayRow, target.file) {
		return cursorCandidate{}
	}
	best := cursorCandidate{}
	for _, pane := range [2]patchPane{
		target.cursor.pane,
		target.cursor.pane.other(),
	} {
		cursor, ok := v.cursorOnRow(row, pane)
		if !ok {
			continue
		}
		line, _ := v.lineAtCursor(cursor)
		candidate := cursorCandidate{
			cursor: cursor,
			rank:   cursorLineMatchRank(line, target.line),
			distance: sourceLineDistance(
				line,
				target.line,
				row-target.cursor.row,
			),
		}
		if candidate.rank == 3 {
			return candidate
		}
		best = candidate.preferredTo(best)
	}
	return best
}

func (v *patchProjection) rowBelongsToFile(
	row displayRow,
	file patch.File,
) bool {
	return row.hunk >= 0 && samePatchFile(v.patch.Files[row.file], file)
}

func (candidate cursorCandidate) betterThan(other cursorCandidate) bool {
	return candidate.rank > other.rank ||
		candidate.rank == other.rank && candidate.distance < other.distance
}

func (candidate cursorCandidate) preferredTo(
	other cursorCandidate,
) cursorCandidate {
	if candidate.betterThan(other) {
		return candidate
	}
	return other
}

func cursorLineMatchRank(line, target patch.Line) int {
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

func sourceLineDistance(line, target patch.Line, rowDistance int) int {
	if line.NewNumber > 0 && target.NewNumber > 0 {
		return absoluteDistance(int(line.NewNumber - target.NewNumber))
	}
	if line.OldNumber > 0 && target.OldNumber > 0 {
		return absoluteDistance(int(line.OldNumber - target.OldNumber))
	}
	return absoluteDistance(rowDistance)
}

func samePatchFile(candidate, target patch.File) bool {
	return candidate.OldPath != "" && candidate.OldPath == target.OldPath ||
		candidate.NewPath != "" && candidate.NewPath == target.NewPath
}

func absoluteDistance(value int) int {
	if value < 0 {
		return -value
	}
	return value
}
