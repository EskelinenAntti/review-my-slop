package diffscreen

import "github.com/eskelinenantti/review-my-slop/internal/patch"

type cursorIdentity struct {
	file   patch.File
	hunk   patch.Hunk
	line   patch.Line
	cursor diffCursor
	valid  bool
}

// replaceView translates interactive state before replacing the row projection.
// Source identity is shared by focus, selection endpoints, and the search
// origin.
func (v *View) replaceView(next *diffView) {
	old := v.view
	leftColumn := v.viewport.LeftColumn
	rowsAbove := v.cursor.row - v.viewport.top
	cursor := captureCursorSourceIdentity(old, v.cursor)
	origin := captureCursorSourceIdentity(old, v.search.from)
	v.viewport = next.newViewport(v.viewport.Width, v.viewport.Height)
	v.viewport = next.scrollHorizontal(v.viewport, leftColumn)
	translated, ok := next.findCursor(cursor)
	if !ok {
		translated, _ = next.firstLineCursor()
	}
	v.cursor = translated
	v.selection = preserveSelection(old, v.selection, next)
	v.view = next
	if next.hasSourceLine(v.cursor) {
		v.viewport.top = max(0, v.cursor.row-rowsAbove)
		v.viewport = next.keepVisible(v.viewport, v.cursor)
	}
	if v.search.active {
		v.search.from = v.cursor
		if translated, ok := next.findCursor(origin); ok {
			v.search.from = translated
		}
		v.applySearchQuery(v.search.query)
	}
}

func captureCursorSourceIdentity(
	v *diffView,
	cursor diffCursor,
) cursorIdentity {
	if !v.hasSourceLine(cursor) {
		return cursorIdentity{}
	}
	row := v.rows[cursor.row]
	file := v.patch.Files[row.file]
	hunk := file.Hunks[row.hunk]
	line := hunk.Lines[v.sourceLineIndexInHunk(row, cursor.pane)]
	return cursorIdentity{
		file:   file,
		hunk:   hunk,
		line:   line,
		cursor: cursor,
		valid:  true,
	}
}

func preserveSelection(
	old *diffView,
	selection *diffSelection,
	next *diffView,
) *diffSelection {
	if selection == nil {
		return nil
	}
	first := captureCursorSourceIdentity(old, selection.First)
	last := captureCursorSourceIdentity(old, selection.Last)
	if !first.valid || !last.valid {
		return nil
	}
	translatedFirst, firstOK := next.findCursor(first)
	translatedLast, lastOK := next.findCursor(last)
	if !firstOK || !lastOK || !sameFile(
		first.file,
		last.file,
	) || first.hunk.Header != last.hunk.Header {
		return nil
	}
	translated := beginSelection(translatedFirst)
	translated, ok := next.extendSelection(translated, translatedLast)
	if !ok {
		return nil
	}
	return &translated
}

// findCursor searches the file by line numbers, then matching text and kind,
// then source proximity. Ties keep the preferred pane.
func (v *diffView) findCursor(target cursorIdentity) (diffCursor, bool) {
	if !target.valid {
		return diffCursor{}, false
	}
	best := cursorMatch{}
	for y := range v.rows {
		candidate := v.findBestRowMatch(y, target)
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

func bestCursorMatch(first, second cursorMatch) cursorMatch {
	if first.outranks(second) {
		return first
	}
	return second
}

func (v *diffView) findBestRowMatch(y int, target cursorIdentity) cursorMatch {
	current := v.rows[y]
	if !v.rowInFile(current, target.file) {
		return cursorMatch{}
	}
	best := cursorMatch{}
	for _, pane := range [2]diffPane{
		target.cursor.pane,
		target.cursor.pane.oppositePane(),
	} {
		candidate, ok := v.sourceCursorAtRow(y, pane)
		if !ok {
			continue
		}
		line, _ := v.line(candidate)
		match := cursorMatch{
			cursor: candidate,
			rank:   scoreLineMatch(line, target.line),
			distance: sourceLineNumberDistance(
				line, target.line, y-target.cursor.row,
			),
		}
		if match.rank == 3 {
			return match
		}
		best = bestCursorMatch(match, best)
	}
	return best
}

func (v *diffView) rowInFile(current entry, file patch.File) bool {
	return current.hunk >= 0 && sameFile(v.patch.Files[current.file], file)
}

func scoreLineMatch(line, target patch.Line) int {
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

func sourceLineNumberDistance(line, target patch.Line, rowDistance int) int {
	if line.NewNumber > 0 && target.NewNumber > 0 {
		return abs(int(line.NewNumber - target.NewNumber))
	}
	if line.OldNumber > 0 && target.OldNumber > 0 {
		return abs(int(line.OldNumber - target.OldNumber))
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
