package ui

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// previewSearch always starts at the saved origin so editing a query does not
// advance through matches.
func previewSearch(screen *Diff, query string) {
	screen.search.query = query
	if query == "" {
		screen.cursor = screen.search.from
		screen.viewport = cursorViewport(screen.view, screen.viewport, screen.cursor)
		screen.search.miss = false
		return
	}
	cursor, ok := findSearchCursor(screen.view, query, screen.search.from)
	screen.search.miss = !ok
	if ok {
		screen.cursor = cursor
		screen.viewport = cursorViewport(screen.view, screen.viewport, cursor)
	}
}

func findSearchCursor(view *diffView, query string,
	cursor diffCursor) (diffCursor, bool) {
	return findSearchCursorDirection(
		view,
		query,
		cursor,
		scanForward,
	)
}

func findSearchCursorDirection(view *diffView, query string,
	cursor diffCursor,
	direction scanDirection) (diffCursor, bool) {
	if query == "" || cursorLine(view, cursor) < 0 {
		return diffCursor{}, false
	}
	query = strings.ToLower(query)
	step := int(direction)
	for count := 0; count < len(view.rows)-1; count++ {
		row := (cursor.row + step + len(view.rows)) % len(view.rows)
		cursor.row = row
		candidate, ok := searchRow(
			view,
			query,
			cursor,
			row,
			direction,
		)
		if ok {
			return candidate, true
		}
	}
	return diffCursor{}, false
}

func searchRow(
	view *diffView,
	query string,
	cursor diffCursor,
	row int,
	direction scanDirection,
) (diffCursor, bool) {
	entry := view.rows[row]
	if entry.kind == lineRow {
		return searchLineRow(view, query, cursor, row)
	}
	text := strings.ToLower(ansi.Strip(entry.text))
	if !strings.Contains(text, query) {
		return diffCursor{}, false
	}
	return searchHeaderCursor(view, row, cursor.pane, direction)
}

func searchLineRow(
	view *diffView,
	query string,
	cursor diffCursor,
	row int,
) (diffCursor, bool) {
	for _, pane := range searchPanes(view, cursor.pane) {
		candidate, ok := cursorAtRow(view, pane, row)
		if !ok {
			continue
		}
		if cursorMatchesQuery(view, candidate, query) {
			return candidate, true
		}
	}
	return diffCursor{}, false
}

func searchPanes(view *diffView,
	pane diffPane) []diffPane {
	panes := []diffPane{pane}
	if view.split {
		panes = append(panes, oppositePane(pane))
	}
	return panes
}

func cursorMatchesQuery(view *diffView,
	cursor diffCursor, query string) bool {
	_, line, ok := patchLineAtCursor(view, cursor)
	return ok && strings.Contains(strings.ToLower(line.Text), query)
}

func searchHeaderCursor(
	view *diffView,
	row int,
	pane diffPane,
	direction scanDirection,
) (diffCursor, bool) {
	step := int(direction)
	for distance := 1; distance <= len(view.rows); distance++ {
		for _, candidateRow := range []int{
			row + step*distance,
			row - step*distance,
		} {
			candidate, ok := searchHeaderCandidate(
				view, row, candidateRow, pane,
			)
			if ok {
				return candidate, true
			}
		}
	}
	return diffCursor{}, false
}

func searchHeaderCandidate(
	view *diffView,
	row, candidateRow int,
	pane diffPane,
) (diffCursor, bool) {
	if candidateRow < 0 || candidateRow >= len(view.rows) ||
		view.rows[candidateRow].file != view.rows[row].file {
		return diffCursor{}, false
	}
	if candidate, ok := cursorAtRow(view, pane, candidateRow); ok {
		return candidate, true
	}
	if view.split {
		return cursorAtRow(
			view, oppositePane(pane), candidateRow,
		)
	}
	return diffCursor{}, false
}

func oppositePane(pane diffPane) diffPane {
	if pane == left {
		return right
	}
	return left
}
