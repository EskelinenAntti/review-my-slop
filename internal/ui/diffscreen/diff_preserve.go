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
// Source identity is shared by focus, selection endpoints, and the search origin.
func (v *View) replaceView(next *diffView) {
	old := v.view
	leftColumn := v.viewport.LeftColumn
	rowsAbove := v.cursor.row - v.viewport.top
	cursor := identify(old, v.cursor)
	origin := identify(old, v.search.from)
	v.viewport = next.newViewport(v.viewport.Width, v.viewport.Height)
	v.viewport = next.scrollHorizontal(v.viewport, leftColumn)
	translated, ok := next.findCursor(cursor)
	if !ok {
		translated, _ = next.first()
	}
	v.cursor = translated
	v.selection = preserveSelection(old, v.selection, next)
	v.view = next
	if next.valid(v.cursor) {
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
	if !v.valid(cursor) {
		return cursorIdentity{}
	}
	row := v.rows[cursor.row]
	file := v.patch.Files[row.file]
	hunk := file.Hunks[row.hunk]
	line := hunk.Lines[v.lineIndex(row, cursor.pane)]
	return cursorIdentity{file: file, hunk: hunk, line: line, cursor: cursor, valid: true}
}

func preserveSelection(old *diffView, selection *diffSelection, next *diffView) *diffSelection {
	if selection == nil {
		return nil
	}
	first := identify(old, selection.First)
	last := identify(old, selection.Last)
	translatedFirst, firstOK := next.findCursor(first)
	translatedLast, lastOK := next.findCursor(last)
	if !firstOK || !lastOK || !sameFile(first.file, last.file) || first.hunk.Header != last.hunk.Header {
		return nil
	}
	translated := next.beginSelection(translatedFirst)
	translated, ok := next.extendSelection(translated, translatedLast)
	if !ok {
		return nil
	}
	return &translated
}
