package diffscreen

import "github.com/eskelinenantti/review-my-slop/internal/patch"

type focusedLineDetails struct {
	file   patch.File
	hunk   patch.Hunk
	line   patch.Line
	cursor diffCursor
	valid  bool
}

// replaceView translates interactive state before replacing the row projection.
// Focus, selection endpoints, and search all retain the saved line.
func (v *View) replaceViewPreservingPosition(next *diffView) {
	old := v.view
	leftColumn := v.viewport.LeftColumn
	rowsAbove := v.cursor.row - v.viewport.top
	cursor := describeFocusedLine(old, v.cursor)
	origin := describeFocusedLine(old, v.search.from)
	v.viewport = next.newVisibleArea(v.viewport.Width, v.viewport.Height)
	v.viewport = next.scrollVisibleAreaSideways(v.viewport, leftColumn)
	translated, ok := next.findSameLineAfterChanges(cursor)
	if !ok {
		translated, _ = next.firstSelectableLine()
	}
	v.cursor = translated
	v.selection = preserveSelection(old, v.selection, next)
	v.view = next
	if next.isChangedLine(v.cursor) {
		v.viewport.top = max(0, v.cursor.row-rowsAbove)
		v.viewport = next.keepFocusedLineVisible(v.viewport, v.cursor)
	}
	if v.search.active {
		v.search.from = v.cursor
		if translated, ok := next.findSameLineAfterChanges(origin); ok {
			v.search.from = translated
		}
		v.previewSearch(v.search.query)
	}
}

func describeFocusedLine(v *diffView, cursor diffCursor) focusedLineDetails {
	if !v.isChangedLine(cursor) {
		return focusedLineDetails{}
	}
	row := v.rows[cursor.row]
	file := v.patch.Files[row.file]
	hunk := file.Hunks[row.hunk]
	line := hunk.Lines[v.findLinePositionInChangeBlock(row, cursor.pane)]
	return focusedLineDetails{
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
	first := describeFocusedLine(old, selection.First)
	last := describeFocusedLine(old, selection.Last)
	if !first.valid || !last.valid {
		return nil
	}
	translatedFirst, firstOK := next.findSameLineAfterChanges(first)
	translatedLast, lastOK := next.findSameLineAfterChanges(last)
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
