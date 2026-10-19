package diffscreen

import (
	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

func beginSelection(cursor diffCursor) diffSelection {
	return diffSelection{First: cursor, Last: cursor}
}

func (s diffSelection) rowBounds() (first, last int) {
	return min(s.First.row, s.Last.row), max(s.First.row, s.Last.row)
}

func (s *diffSelection) contains(cursor diffCursor) bool {
	if s == nil {
		return false
	}
	if s.First.row == s.Last.row && s.First.pane != s.Last.pane {
		return cursor.row == s.First.row &&
			(cursor.pane == s.First.pane || cursor.pane == s.Last.pane)
	}
	first, last := s.rowBounds()
	return cursor.pane == s.First.pane &&
		cursor.row >= first && cursor.row <= last
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
	first, last := selection.rowBounds()
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
