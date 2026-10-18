package diffscreen

import "github.com/eskelinenantti/review-my-slop/internal/patch"

func sameFilePaths(
	candidateOld, candidateNew, targetOld, targetNew string,
) bool {
	oldPathMatches := candidateOld != "" && candidateOld == targetOld
	newPathMatches := candidateNew != "" && candidateNew == targetNew
	return oldPathMatches || newPathMatches
}

type cursorIdentity struct {
	oldPath, newPath string
	hunkHeader       string
	line             patch.Line
	cursor           diffCursor
	valid            bool
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
	cursor   diffCursor
	rank     cursorMatchRank
	distance int
}

type cursorMatchRank uint8

const (
	noCursorMatch cursorMatchRank = iota
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

func lineMatchRank(line, target patch.Line) cursorMatchRank {
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

func abs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

// replaceView translates interactive state before replacing the row projection.
// Source identity is shared by focus, selection endpoints, and the search
// origin.
func (v *View) replaceView(next *diffView) {
	old := v.view
	leftColumn := v.viewport.leftColumn
	rowsAbove := v.cursor.row - v.viewport.top
	cursor := identify(old, v.cursor)
	origin := identify(old, v.search.from)
	v.viewport = next.newViewport(v.viewport.width, v.viewport.height)
	v.viewport = next.scrollHorizontal(v.viewport, leftColumn)
	translated, ok := next.findCursor(cursor)
	if !ok {
		translated, _ = next.first()
	}
	v.cursor = translated
	v.selection = preserveSelection(old, v.selection, next)
	v.view = next
	if next.validCursor(v.cursor) {
		v.viewport.top = max(0, v.cursor.row-rowsAbove)
		v.viewport = next.keepVisible(v.viewport, v.cursor)
	}
	if v.search.active {
		v.search.from = v.cursor
		if translated, ok := next.findCursor(origin); ok {
			v.search.from = translated
		}
		v.previewSearch(v.search.query)
	}
}

func identify(v *diffView, cursor diffCursor) cursorIdentity {
	if !v.validCursor(cursor) {
		return cursorIdentity{}
	}
	row := v.rows[cursor.row]
	file := v.patch.Files[row.file]
	hunk := file.Hunks[row.hunk]
	line := hunk.Lines[v.hunkLineIndex(row, cursor.pane)]
	return cursorIdentity{
		oldPath:    file.OldPath,
		newPath:    file.NewPath,
		hunkHeader: hunk.Header,
		line:       line,
		cursor:     cursor,
		valid:      true,
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
	first := identify(old, selection.First)
	last := identify(old, selection.Last)
	if !first.valid || !last.valid {
		return nil
	}
	translatedFirst, firstOK := next.findCursor(first)
	translatedLast, lastOK := next.findCursor(last)
	if !firstOK || !lastOK || !sameFilePaths(
		first.oldPath,
		first.newPath,
		last.oldPath,
		last.newPath,
	) || first.hunkHeader != last.hunkHeader {
		return nil
	}
	translated := beginSelection(translatedFirst)
	translated, ok := next.extendSelection(translated, translatedLast)
	if !ok {
		return nil
	}
	return &translated
}
