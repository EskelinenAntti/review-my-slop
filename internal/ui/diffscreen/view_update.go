package diffscreen

import "github.com/eskelinenantti/review-my-slop/internal/patch"

type cursorIdentity struct {
	file   patch.File
	hunk   patch.Hunk
	line   patch.Line
	cursor patchCursor
	valid  bool
}

// replaceProjection translates interactive state before projection replacement.
// Source identity is shared by focus, selection endpoints, and the search
// origin.
func (v *View) replaceProjection(next *patchProjection) {
	old := v.projection
	leftColumn := v.viewport.LeftColumn
	rowsAbove := v.cursor.row - v.viewport.top
	cursor := identifyCursor(old, v.cursor)
	origin := identifyCursor(old, v.search.from)
	v.viewport = next.newViewport(v.viewport.Width, v.viewport.Height)
	v.viewport = next.scrollHorizontal(v.viewport, leftColumn)
	translated, ok := next.restoreCursor(cursor)
	if !ok {
		translated, _ = next.firstCursor()
	}
	v.cursor = translated
	v.selection = preserveRange(old, v.selection, next)
	v.projection = next
	if next.validCursor(v.cursor) {
		v.viewport.top = max(0, v.cursor.row-rowsAbove)
		v.viewport = next.keepCursorVisible(v.viewport, v.cursor)
	}
	if v.search.active {
		v.search.from = v.cursor
		if translated, ok := next.restoreCursor(origin); ok {
			v.search.from = translated
		}
		v.previewSearch(v.search.query)
	}
}

func identifyCursor(v *patchProjection, cursor patchCursor) cursorIdentity {
	if !v.validCursor(cursor) {
		return cursorIdentity{}
	}
	row := v.rows[cursor.row]
	file := v.patch.Files[row.file]
	hunk := file.Hunks[row.hunk]
	line := hunk.Lines[v.sourceLineIndex(row, cursor.pane)]
	return cursorIdentity{
		file:   file,
		hunk:   hunk,
		line:   line,
		cursor: cursor,
		valid:  true,
	}
}

func preserveRange(
	old *patchProjection,
	selection *patchRange,
	next *patchProjection,
) *patchRange {
	if selection == nil {
		return nil
	}
	firstIdentity := identifyCursor(old, selection.First)
	lastIdentity := identifyCursor(old, selection.Last)
	if !firstIdentity.valid || !lastIdentity.valid {
		return nil
	}
	translatedFirst, firstOK := next.restoreCursor(firstIdentity)
	translatedLast, lastOK := next.restoreCursor(lastIdentity)
	if !firstOK || !lastOK || !samePatchFile(
		firstIdentity.file,
		lastIdentity.file,
	) || firstIdentity.hunk.Header != lastIdentity.hunk.Header {
		return nil
	}
	translated := beginRange(translatedFirst)
	translated, ok := next.extendRange(translated, translatedLast)
	if !ok {
		return nil
	}
	return &translated
}
