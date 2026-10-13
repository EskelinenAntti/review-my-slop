package diffscreen

import (
	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

func beginRange(cursor patchCursor) patchRange {
	return patchRange{First: cursor, Last: cursor}
}

func cursorIsSelected(selection *patchRange, cursor patchCursor) bool {
	if selection == nil {
		return false
	}
	firstCursor, lastCursor := selection.First, selection.Last
	if firstCursor.row == lastCursor.row &&
		firstCursor.pane != lastCursor.pane {
		return cursor.row == firstCursor.row &&
			(cursor.pane == firstCursor.pane || cursor.pane == lastCursor.pane)
	}
	if firstCursor.pane != cursor.pane {
		return false
	}
	return cursor.row >= min(
		firstCursor.row,
		lastCursor.row,
	) && cursor.row <= max(firstCursor.row, lastCursor.row)
}

func (v *patchProjection) extendRange(
	selection patchRange,
	cursor patchCursor,
) (patchRange, bool) {
	if !v.validCursor(selection.First) || !v.validCursor(cursor) {
		return selection, false
	}
	firstRow := v.rows[selection.First.row]
	lastRow := v.rows[cursor.row]
	if firstRow.file != lastRow.file || firstRow.hunk != lastRow.hunk {
		return selection, false
	}
	selection.Last = cursor
	return selection, true
}

func (v *patchProjection) selectedLines(selection patchRange) []patch.Line {
	if _, ok := v.extendRange(selection, selection.Last); !ok {
		return nil
	}
	firstRow, lastRow := min(
		selection.First.row,
		selection.Last.row,
	), max(selection.First.row, selection.Last.row)
	if firstRow == lastRow && selection.First.pane != selection.Last.pane {
		return v.selectedPaneLines(selection)
	}
	lines := make([]patch.Line, 0, lastRow-firstRow+1)
	for y := firstRow; y <= lastRow; y++ {
		pane := selection.selectedPaneAt(y)
		if line, ok := v.lineAtCursor(patchCursor{row: y, pane: pane}); ok {
			lines = append(lines, line)
		}
	}
	return lines
}

func (s patchRange) selectedPaneAt(row int) patchPane {
	if row == s.Last.row {
		return s.Last.pane
	}
	return s.First.pane
}

func (v *patchProjection) selectedPaneLines(selection patchRange) []patch.Line {
	firstLine, _ := v.lineAtCursor(selection.First)
	lastLine, _ := v.lineAtCursor(selection.Last)
	if firstLine == lastLine {
		return []patch.Line{firstLine}
	}
	return []patch.Line{firstLine, lastLine}
}
