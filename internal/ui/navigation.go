package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	patches "github.com/eskelinenantti/review-my-slop/internal/patch"
)

func viewportBottom(view *diffView, height int) int {
	bottom := max(0, len(view.rows)-height)
	if height > 1 && bottom < len(
		view.rows,
	) && view.rows[bottom].kind != fileRow {
		bottom++
	}
	return bottom
}

func viewportLeft(view *diffView,
	viewport diffViewport) int {
	contentWidth := max(1, viewport.Width-14)
	extraWidth := 0
	if view.split {
		contentWidth, extraWidth = max(1, (viewport.Width-3)/2-6), 2
	}
	longest := 0
	for _, row := range view.rows {
		if row.kind != lineRow {
			continue
		}
		text := ansi.Strip(row.text + row.left + row.right)
		text = strings.ReplaceAll(text, "\t", "    ")
		longest = max(longest, lipgloss.Width(text)+extraWidth)
	}
	return max(0, longest-contentWidth)
}

func clampViewport(view *diffView,
	viewport diffViewport) diffViewport {
	viewport.top = max(0, min(viewport.top, viewportBottom(
		view,
		viewport.Height,
	)))
	viewport.LeftColumn = max(0, min(
		viewport.LeftColumn,
		viewportLeft(view, viewport),
	))
	return viewport
}

func cursorViewport(
	view *diffView,
	viewport diffViewport,
	cursor diffCursor,
) diffViewport {
	viewport = clampViewport(view, viewport)
	if cursorLine(view, cursor) < 0 {
		return viewport
	}
	for range 2 {
		height := max(1, viewportContentHeight(view, viewport))
		viewport.top = cursorViewportTop(cursor, viewport, height)
		viewport = clampViewport(view, viewport)
	}
	return viewport
}

func cursorViewportTop(
	cursor diffCursor,
	viewport diffViewport,
	height int,
) int {
	if cursor.row < viewport.top {
		return cursor.row
	}
	if cursor.row >= viewport.top+height {
		return cursor.row - height + 1
	}
	return viewport.top
}

func moveFileOrPane(screen *Diff, motion Motion) {
	switch motion {
	case PreviousFile:
		screen.ClearSelection()
		moveFile(screen, scanBackward)
	case NextFile:
		screen.ClearSelection()
		moveFile(screen, scanForward)
	case OldPane:
		switchPane(screen, left)
	case NewPane:
		switchPane(screen, right)
	case OtherPane:
		switchPane(screen, oppositePane(screen.cursor.pane))
	default:
		return
	}
}

func moveFile(screen *Diff,
	direction scanDirection) {
	cursor, ok := jumpFile(
		screen.view,
		screen.cursor,
		direction,
	)
	if ok {
		screen.cursor = cursor
		screen.viewport = cursorVisibleViewport(
			cursor,
			screen.view,
			screen.viewport,
		)
	}
}

func lastCursor(view *diffView) (diffCursor,
	bool) {
	for _, pane := range []diffPane{right, left} {
		for row := len(view.rows) - 1; row >= 0; row-- {
			cursor := diffCursor{row: row, pane: pane}
			if cursorLine(view, cursor) >= 0 {
				return cursor, true
			}
		}
	}
	return diffCursor{}, false
}

func movePage(screen *Diff,
	direction scanDirection) {
	viewport, cursor := pageTarget(
		screen.view,
		screen.viewport,
		screen.cursor,
		direction,
	)
	if screen.selection != nil {
		selection, ok := selectionForDrag(
			screen.view,
			screen.selection.First,
			cursor,
		)
		if !ok {
			return
		}
		screen.selection = &selection
	}
	screen.viewport, screen.cursor = viewport, cursor
}

func pageTarget(view *diffView,
	viewport diffViewport, cursor diffCursor,
	direction scanDirection) (diffViewport,
	diffCursor) {
	if cursorLine(view, cursor) < 0 {
		return viewport, cursor
	}
	distance := int(direction) * max(1, viewport.Height/2)
	viewport.top += distance
	viewport = clampViewport(view, viewport)
	target := min(len(view.rows)-1, max(0, cursor.row+distance))
	candidate, ok := nearestCursor(
		view,
		viewport,
		cursor.pane,
		target,
		direction,
	)
	if ok {
		cursor = candidate
	}
	return viewport, cursor
}

func moveLine(screen *Diff,
	direction scanDirection) {
	cursor, ok := scanCursor(screen.view, screen.cursor, direction)
	if !ok {
		return
	}
	if screen.selection != nil {
		selection, ok := selectionForDrag(
			screen.view,
			screen.selection.First,
			cursor,
		)
		if !ok {
			return
		}
		screen.selection = &selection
	}
	screen.cursor = cursor
	screen.viewport = cursorViewport(
		screen.view,
		screen.viewport,
		cursor,
	)
}

func switchPane(screen *Diff, pane diffPane) {
	cursor, ok := closestCursorInPane(pane, screen.view, screen.cursor)
	if !ok {
		return
	}
	var selection *diffSelection
	if screen.selection != nil {
		mapped, ok := selectionInPane(
			pane,
			screen.view,
			*screen.selection,
		)
		if !ok {
			return
		}
		selection = &mapped
	}
	screen.cursor = cursor
	screen.selection = selection
	screen.viewport = cursorVisibleViewport(
		cursor,
		screen.view,
		screen.viewport,
	)
}

func selectionInPane(pane diffPane,
	view *diffView,
	selection diffSelection) (diffSelection, bool) {
	first, firstOK := closestCursorInPane(pane, view, selection.First)
	last, lastOK := closestCursorInPane(pane, view, selection.Last)
	if !firstOK || !lastOK {
		return diffSelection{}, false
	}
	firstRow := view.rows[first.row]
	lastRow := view.rows[last.row]
	if firstRow.file != lastRow.file || firstRow.hunk != lastRow.hunk {
		return diffSelection{}, false
	}
	return diffSelection{First: first, Last: last}, true
}

func alignmentOffset(alignment Alignment, height int) int {
	switch alignment {
	case Center:
		return height / 2
	case Bottom:
		return height - 1
	default:
		return 0
	}
}

func viewportHeight(view *diffView,
	viewport diffViewport) int {
	height := viewport.Height
	if height > 1 && viewport.top >= 0 && viewport.top < len(
		view.rows,
	) && view.rows[viewport.top].kind != fileRow {
		height--
	}
	return max(1, height)
}

func cursorVisible(view *diffView,
	viewport diffViewport, cursor diffCursor) bool {
	if cursorLine(view, cursor) < 0 {
		return false
	}
	return cursor.row >= viewport.top &&
		cursor.row < viewport.top+viewportHeight(
			view,
			viewport,
		)
}

func nearestCursor(view *diffView,
	viewport diffViewport, pane diffPane, target int,
	direction scanDirection) (diffCursor, bool) {
	height := viewportHeight(view, viewport)
	for distance := 0; distance < height; distance++ {
		for _, row := range []int{target + int(
			direction,
		)*distance, target - int(direction)*distance} {
			cursor := diffCursor{row: row, pane: pane}
			if cursorVisible(view, viewport, cursor) {
				return cursor, true
			}
		}
	}
	return diffCursor{}, false
}

func scrollDirection(row, target int) scanDirection {
	if row > target {
		return scanBackward
	}
	return scanForward
}

func nearestScrollCursor(
	view *diffView,
	viewport diffViewport,
	cursor diffCursor,
	target int,
	direction scanDirection,
) (diffCursor, bool) {
	candidate, ok := nearestCursor(
		view, viewport, cursor.pane, target, direction,
	)
	if ok || !view.split {
		return candidate, ok
	}
	return nearestCursor(
		view, viewport, oppositePane(cursor.pane), target, direction,
	)
}

func scrollSelection(
	view *diffView,
	selection *diffSelection,
	cursor diffCursor,
) (*diffSelection, bool) {
	if selection == nil {
		return nil, true
	}
	updated, ok := selectionForDrag(view, selection.First, cursor)
	if !ok {
		return selection, false
	}
	return &updated, true
}

func patchLineAtCursor(view *diffView,
	cursor diffCursor) (patches.File, patches.Line, bool) {
	lineIndex := cursorLine(view, cursor)
	if lineIndex < 0 {
		return patches.File{}, patches.Line{}, false
	}
	row := view.rows[cursor.row]
	file := view.patch.Files[row.file]
	line := file.Hunks[row.hunk].Lines[lineIndex]
	return file, line, true
}

func selectionFile(view *diffView,
	selection diffSelection) (patches.File, bool) {
	file, _, firstOK := patchLineAtCursor(view, selection.First)
	_, _, lastOK := patchLineAtCursor(view, selection.Last)
	if !firstOK || !lastOK {
		return file, false
	}
	firstRow := view.rows[selection.First.row]
	lastRow := view.rows[selection.Last.row]
	return file, firstRow.file == lastRow.file && firstRow.hunk == lastRow.hunk
}

func linesInSelection(view *diffView,
	selection diffSelection) []patches.Line {
	_, first, firstOK := patchLineAtCursor(view, selection.First)
	_, last, lastOK := patchLineAtCursor(view, selection.Last)
	if !firstOK || !lastOK {
		return nil
	}
	if selectionCrossesPanes(selection) {
		return linesAcrossPanes(first, last)
	}
	return linesAcrossRows(view, selection)
}

func selectionCrossesPanes(selection diffSelection) bool {
	return selection.First.row == selection.Last.row &&
		selection.First.pane != selection.Last.pane
}

func linesAcrossPanes(first, last patches.Line) []patches.Line {
	if first == last {
		return []patches.Line{first}
	}
	return []patches.Line{first, last}
}

func linesAcrossRows(view *diffView,
	selection diffSelection) []patches.Line {
	start := min(selection.First.row, selection.Last.row)
	end := max(selection.First.row, selection.Last.row)
	lines := make([]patches.Line, 0, end-start+1)
	for row := start; row <= end; row++ {
		cursor := diffCursor{
			row:  row,
			pane: selectionPaneAtRow(selection, row),
		}
		if _, line, ok := patchLineAtCursor(view, cursor); ok {
			lines = append(lines, line)
		}
	}
	return lines
}

func selectionPaneAtRow(selection diffSelection,
	row int) diffPane {
	if row == selection.Last.row {
		return selection.Last.pane
	}
	return selection.First.pane
}

func lineNumberAtCursor(cursor diffCursor,
	v *diffView,

) int {
	var current = v.rows[cursor.row]
	if current.kind != lineRow {
		return -1
	}
	if !v.split || cursor.pane == right {
		return current.rightLine
	}
	return current.leftLine
}

func translateCursorIdentity(
	next *diffView,
	origin cursorIdentity,
) (diffCursor, bool) {
	return findCursorMatch(next, origin)
}

func lineForCursor(candidate diffCursor,
	next *diffView) (patches.Line, bool) {
	_, line, ok := patchLineAtCursor(next, candidate)
	return line, ok
}

func firstCursor(next *diffView) (diffCursor,
	bool) {
	for _, pane := range []diffPane{right, left} {
		for row := range next.rows {
			cursor := diffCursor{row: row, pane: pane}
			if cursorLine(next, cursor) >= 0 {
				return cursor, true
			}
		}
	}
	return diffCursor{}, false
}

func cursorVisibleViewport(cursor diffCursor,
	v *diffView,
	viewport diffViewport) diffViewport {
	return cursorViewport(v, viewport, cursor)
}

func cursorAtRow(next *diffView, pane diffPane,
	y int) (diffCursor, bool) {
	cursor := diffCursor{row: y, pane: pane}
	return cursor, cursorLine(next, cursor) >= 0
}

func scanCursor(view *diffView, start diffCursor,
	direction scanDirection) (diffCursor, bool) {
	if cursorLine(view, start) < 0 || direction == 0 {
		return diffCursor{}, false
	}
	step := int(direction)
	for row := start.row + step; row >= 0 && row < len(view.rows); row += step {
		if cursor, ok := cursorAtRow(view, start.pane, row); ok {
			return cursor, true
		}
	}
	return diffCursor{}, false
}

func jumpFile(
	view *diffView,
	cursor diffCursor,
	direction scanDirection,
) (diffCursor, bool) {
	file, _, ok := patchLineAtCursor(view, cursor)
	if !ok {
		return diffCursor{}, false
	}
	for {
		next, changed, ok := nextFileCursor(
			view, cursor, direction, file,
		)
		if !ok {
			return diffCursor{}, false
		}
		if changed {
			return next, true
		}
		cursor = next
	}
}

func nextFileCursor(
	view *diffView,
	cursor diffCursor,
	direction scanDirection,
	file patches.File,
) (nextCursor diffCursor, changed, valid bool) {
	next, ok := scanCursor(view, cursor, direction)
	if !ok {
		return diffCursor{}, false, false
	}
	changed, valid = cursorMovedToFile(view, next, file)
	return next, changed, valid
}

func cursorMovedToFile(
	view *diffView,
	cursor diffCursor,
	file patches.File,
) (changed, valid bool) {
	nextFile, _, valid := patchLineAtCursor(view, cursor)
	if !valid {
		return false, false
	}
	return nextFile.OldPath != file.OldPath || nextFile.NewPath != file.NewPath,
		true
}

func viewportClamp(v *diffView,
	viewport *diffViewport) diffViewport {
	return clampViewport(v, *viewport)
}

func contentWidth(v *diffView,
	viewport *diffViewport) int {
	return viewportLeft(v, *viewport)
}

func closestCursorInPane(
	pane diffPane,
	view *diffView,
	cursor diffCursor,
) (diffCursor, bool) {
	if !view.split || cursorLine(view, cursor) < 0 {
		return diffCursor{}, false
	}
	if candidate, ok := cursorAtRow(view, pane, cursor.row); ok {
		return candidate, true
	}
	if candidate, ok := previousCursorInFile(view, pane, cursor); ok {
		return candidate, true
	}
	return nextCursor(view, pane, cursor.row)
}

func previousCursorInFile(view *diffView,
	pane diffPane, cursor diffCursor) (diffCursor,
	bool) {
	file := view.rows[cursor.row].file
	for row := cursor.row - 1; row >= 0; row-- {
		if view.rows[row].file != file {
			return diffCursor{}, false
		}
		if candidate, ok := cursorAtRow(view, pane, row); ok {
			return candidate, true
		}
	}
	return diffCursor{}, false
}

func nextCursor(view *diffView, pane diffPane,
	row int) (diffCursor, bool) {
	for row++; row < len(view.rows); row++ {
		if candidate, ok := cursorAtRow(view, pane, row); ok {
			return candidate, true
		}
	}
	return diffCursor{}, false
}

func computeDiffscreenViewport(
	v *diffView,
	width, height int,
) diffViewport {
	viewport := diffViewport{
		Width:  max(1, width),
		Height: max(1, height),
	}
	viewport.top = min(viewport.top, maximumDiffscreenTop(v, viewport.Height))
	viewport.LeftColumn = min(viewport.LeftColumn, maximumDiffscreenLeft(
		v,
		viewport.Width,
	))
	return viewport
}

func maximumDiffscreenTop(view *diffView, height int) int {
	end := max(0, len(view.rows)-height)
	if height > 1 && end < len(
		view.rows,
	) && view.rows[end].kind != fileRow {
		end++
	}
	return end
}

func maximumDiffscreenLeft(view *diffView, width int) int {
	contentWidth, extra := max(1, width-14), 0
	if view.split {
		contentWidth, extra = max(1, (width-3)/2-6), 2
	}
	longest := 0
	for _, row := range view.rows {
		if row.kind == lineRow {
			longest = max(longest, diffRowDisplayWidth(row)+extra)
		}
	}
	return max(0, longest-contentWidth)
}

func diffRowDisplayWidth(row entry) int {
	text := ansi.Strip(row.text + row.left + row.right)
	return lipgloss.Width(strings.ReplaceAll(text, "\t", "    "))
}

func findInitialDiffCursor(view *Diff) (diffCursor, bool) {
	if cursor, ok := findCursorInPane(view.view, right); ok {
		return cursor, true
	}
	return findCursorInPane(view.view, left)
}

func findCursorInPane(
	view *diffView,
	pane diffPane,
) (diffCursor, bool) {
	for row := range view.rows {
		cursor := diffCursor{row: row, pane: pane}
		if diffCursorHasLine(view, cursor) {
			return cursor, true
		}
	}
	return diffCursor{}, false
}

func diffCursorHasLine(
	view *diffView,
	cursor diffCursor,
) bool {
	if cursor.row < 0 || cursor.row >= len(view.rows) {
		return false
	}
	entry := view.rows[cursor.row]
	if entry.kind != lineRow {
		return false
	}
	if !view.split || cursor.pane == right {
		return entry.rightLine >= 0
	}
	return entry.leftLine >= 0
}
