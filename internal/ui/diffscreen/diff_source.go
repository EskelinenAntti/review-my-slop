package diffscreen

import "github.com/eskelinenantti/review-my-slop/internal/patch"

type sourcePosition struct {
	file   patch.File
	hunk   patch.Hunk
	line   patch.Line
	cursor diffCursor
	valid  bool
}

func (v *projection) sourceAt(cursor diffCursor) sourcePosition {
	if !v.hasSourceLine(cursor) {
		return sourcePosition{}
	}
	row := v.rows[cursor.row]
	file := v.patch.Files[row.file]
	hunk := file.Hunks[row.hunk]
	line := hunk.Lines[v.lineIndex(row, cursor.pane)]
	return sourcePosition{file: file, hunk: hunk, line: line, cursor: cursor, valid: true}
}

// restoreCursor prioritizes line numbers, then matching text and kind, then kind,
// then any line in the same hunk. Ties keep the closest row and preferred pane.
func (v *projection) restoreCursor(target sourcePosition) (diffCursor, bool) {
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
			line := v.sourceAt(candidate).line
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

func abs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}
