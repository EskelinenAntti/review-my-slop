package tui

type displayedRow interface {
	render(Model) string
	sourceRow() *row
	cursorRow(pane) (*row, bool)
	nextRow() (displayedRow, bool)
	previousRow() (displayedRow, bool)
}

type diffLayout interface {
	displayedRowForSource(*row) (displayedRow, bool)
}

type stackedLayout struct{}

func newStackedLayout() stackedLayout {
	return stackedLayout{}
}

func (stackedLayout) displayedRowForSource(source *row) (displayedRow, bool) {
	if source == nil {
		return nil, false
	}
	return source, true
}

func (current *row) render(model Model) string {
	return model.renderStackedRow(current)
}

func (current *row) sourceRow() *row {
	return current
}

func (current *row) cursorRow(_ pane) (*row, bool) {
	return current, current != nil && current.kind == rowCode
}

func (current *row) nextRow() (displayedRow, bool) {
	if current == nil || current.next == nil {
		return nil, false
	}
	return current.next, true
}

func (current *row) previousRow() (displayedRow, bool) {
	if current == nil || current.previous == nil {
		return nil, false
	}
	return current.previous, true
}

func displayedRowsStartingAt(layout diffLayout, first *row, count int) []displayedRow {
	if count <= 0 {
		return nil
	}
	current, ok := layout.displayedRowForSource(first)
	if !ok {
		return nil
	}
	rows := make([]displayedRow, 0, count)
	for len(rows) < count {
		rows = append(rows, current)
		next, ok := current.nextRow()
		if !ok {
			break
		}
		current = next
	}
	return rows
}

func countDisplayedRowsBetween(layout diffLayout, first, last *row) int {
	current, ok := layout.displayedRowForSource(first)
	if !ok {
		return 0
	}
	target, ok := layout.displayedRowForSource(last)
	if !ok {
		return 0
	}
	count := 0
	for current != target {
		next, ok := current.nextRow()
		if !ok {
			return 0
		}
		current = next
		count++
	}
	return count
}

func sourceRowBefore(layout diffLayout, source *row, count int) *row {
	current, ok := layout.displayedRowForSource(source)
	if !ok {
		return nil
	}
	for count > 0 {
		previous, ok := current.previousRow()
		if !ok {
			break
		}
		current = previous
		count--
	}
	return current.sourceRow()
}

func countSourceRowsBefore(rows rowList, target *row) int {
	count := 0
	for current := rows.first; current != nil && current != target; current = current.next {
		count++
	}
	return count
}

func sourceRowAfter(first *row, count int) *row {
	current := first
	for current != nil && count > 0 && current.next != nil {
		current = current.next
		count--
	}
	return current
}

func nextCursorRow(layout diffLayout, source *row, targetPane pane) (*row, bool) {
	current, ok := layout.displayedRowForSource(source)
	if !ok {
		return nil, false
	}
	for {
		current, ok = current.nextRow()
		if !ok {
			return nil, false
		}
		if cursor, ok := current.cursorRow(targetPane); ok {
			return cursor, true
		}
	}
}

func previousCursorRow(layout diffLayout, source *row, targetPane pane) (*row, bool) {
	current, ok := layout.displayedRowForSource(source)
	if !ok {
		return nil, false
	}
	for {
		current, ok = current.previousRow()
		if !ok {
			return nil, false
		}
		if cursor, ok := current.cursorRow(targetPane); ok {
			return cursor, true
		}
	}
}

type displayedRowStep func(displayedRow) (displayedRow, bool)

func walkDisplayedRows(layout diffLayout, source *row, count int, step displayedRowStep) *row {
	current, ok := layout.displayedRowForSource(source)
	if !ok {
		return nil
	}
	for count > 0 {
		next, ok := step(current)
		if !ok {
			break
		}
		current = next
		count--
	}
	return current.sourceRow()
}

func viewportTopFillingHeight(layout diffLayout, top *row, height int) *row {
	visible := displayedRowsStartingAt(layout, top, height)
	missing := height - len(visible)
	if missing <= 0 || len(visible) == 0 {
		return top
	}
	return walkDisplayedRows(layout, top, missing, displayedRow.previousRow)
}

func viewportTopKeepingCursorVisible(layout diffLayout, top, cursor *row, height int) *row {
	target, ok := layout.displayedRowForSource(cursor)
	if !ok {
		return nil
	}
	for _, current := range displayedRowsStartingAt(layout, top, height) {
		if current == target {
			return viewportTopFillingHeight(layout, top, height)
		}
	}

	current, ok := layout.displayedRowForSource(top)
	for ok {
		if current == target {
			return viewportTopFillingHeight(layout, cursor, height)
		}
		current, ok = current.previousRow()
	}
	return viewportTopFillingHeight(layout, sourceRowBefore(layout, cursor, height-1), height)
}

func displayedRowAfter(layout diffLayout, source *row, count int) (displayedRow, bool) {
	current, ok := layout.displayedRowForSource(source)
	if !ok {
		return nil, false
	}
	for count > 0 {
		next, ok := current.nextRow()
		if !ok {
			break
		}
		current = next
		count--
	}
	return current, true
}

func cursorRowNear(target displayedRow, visible []displayedRow, targetPane pane, firstStep, fallbackStep displayedRowStep) (*row, bool) {
	withinViewport := make(map[displayedRow]struct{}, len(visible))
	for _, current := range visible {
		withinViewport[current] = struct{}{}
	}
	if cursor, ok := firstSelectableRowInDirection(target, withinViewport, targetPane, firstStep); ok {
		return cursor, true
	}
	return firstSelectableRowInDirection(target, withinViewport, targetPane, fallbackStep)
}

func firstSelectableRowInDirection(start displayedRow, visibleRows map[displayedRow]struct{}, targetPane pane, advance displayedRowStep) (*row, bool) {
	current := start
	for {
		if _, ok := visibleRows[current]; !ok {
			return nil, false
		}
		if cursor, ok := current.cursorRow(targetPane); ok {
			return cursor, true
		}
		next, ok := advance(current)
		if !ok {
			return nil, false
		}
		current = next
	}
}
