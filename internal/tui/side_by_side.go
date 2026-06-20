package tui

import "github.com/eskelinenantti/review-my-slop/internal/review"

type pane uint8

const (
	paneLeft pane = iota
	paneRight
)

const minimumSideBySideWidth = 100

type sideBySideRow struct {
	primaryRow *row
	leftRow    *row
	rightRow   *row
	previous   *sideBySideRow
	next       *sideBySideRow
}

type sideBySideProjection struct {
	sourceRows               rowList
	sideBySideRows           []*sideBySideRow
	sideBySideRowBySourceRow map[*row]*sideBySideRow
	lastRow                  *sideBySideRow
}

func newSideBySideProjection(rows rowList) sideBySideProjection {
	projection := sideBySideProjection{
		sourceRows:               rows,
		sideBySideRowBySourceRow: make(map[*row]*sideBySideRow, rows.count),
	}

	for current := rows.first; current != nil; {
		if current.kind != rowCode {
			projection.appendRow(&sideBySideRow{primaryRow: current, leftRow: current, rightRow: current})
			current = current.next
			continue
		}

		switch current.line.Kind {
		case review.LineContext:
			projection.appendRow(&sideBySideRow{primaryRow: current, leftRow: current, rightRow: current})
			current = current.next
		case review.LineAdded:
			projection.appendRow(&sideBySideRow{primaryRow: current, rightRow: current})
			current = current.next
		case review.LineRemoved:
			removedEnd := projection.changeBlockEnd(current, review.LineRemoved)
			addedStart := removedEnd
			addedEnd := addedStart
			if projection.sameHunkKind(addedStart, current, review.LineAdded) {
				addedEnd = projection.changeBlockEnd(addedStart, review.LineAdded)
			}

			left, right := current, addedStart
			for left != removedEnd || right != addedEnd {
				var leftRow, rightRow *row
				if left != removedEnd {
					leftRow = left
					left = left.next
				}
				if right != addedEnd {
					rightRow = right
					right = right.next
				}
				primaryRow := leftRow
				if primaryRow == nil {
					primaryRow = rightRow
				}
				projection.appendRow(&sideBySideRow{primaryRow: primaryRow, leftRow: leftRow, rightRow: rightRow})
			}
			current = addedEnd
		}
	}
	return projection
}

func (projection *sideBySideProjection) appendRow(current *sideBySideRow) {
	if projection.lastRow != nil {
		projection.lastRow.next = current
		current.previous = projection.lastRow
	}
	projection.sideBySideRows = append(projection.sideBySideRows, current)
	projection.lastRow = current
	if current.leftRow != nil {
		projection.sideBySideRowBySourceRow[current.leftRow] = current
	}
	if current.rightRow != nil {
		projection.sideBySideRowBySourceRow[current.rightRow] = current
	}
}

func (projection sideBySideProjection) changeBlockEnd(start *row, kind review.LineKind) *row {
	current := start
	for projection.sameHunkKind(current, start, kind) {
		current = current.next
	}
	return current
}

func (sideBySideProjection) sameHunkKind(candidate, current *row, kind review.LineKind) bool {
	return candidate != nil && current != nil &&
		candidate.kind == rowCode &&
		candidate.file == current.file &&
		candidate.hunk == current.hunk &&
		candidate.line.Kind == kind
}

func (projection sideBySideProjection) displayedRowForSource(source *row) (displayedRow, bool) {
	current, ok := projection.sideBySideRowBySourceRow[source]
	return current, ok
}

func (current *sideBySideRow) render(model Model) string {
	return model.renderSideBySideRow(current)
}

func (current *sideBySideRow) sourceRow() *row {
	if current == nil {
		return nil
	}
	return current.primaryRow
}

func (current *sideBySideRow) cursorRow(target pane) (*row, bool) {
	if current == nil {
		return nil, false
	}
	targetRow := current.rightRow
	if target == paneLeft {
		targetRow = current.leftRow
	}
	return targetRow, targetRow != nil && targetRow.kind == rowCode
}

func (current *sideBySideRow) nextRow() (displayedRow, bool) {
	if current == nil || current.next == nil {
		return nil, false
	}
	return current.next, true
}

func (current *sideBySideRow) previousRow() (displayedRow, bool) {
	if current == nil || current.previous == nil {
		return nil, false
	}
	return current.previous, true
}

func sideBySideRowBetween(candidate, first, last *sideBySideRow) bool {
	if candidate == nil || first == nil || last == nil {
		return false
	}
	first, last = orderedSideBySideRows(first, last)
	for current := first; current != nil; current = current.next {
		if current == candidate {
			return true
		}
		if current == last {
			return false
		}
	}
	return false
}

func orderedSideBySideRows(first, last *sideBySideRow) (*sideBySideRow, *sideBySideRow) {
	for current := first; current != nil; current = current.next {
		if current == last {
			return first, last
		}
	}
	return last, first
}

func (projection sideBySideProjection) preferredCursor(source *row, preferred pane) (*row, bool) {
	current, ok := projection.sideBySideRowBySourceRow[source]
	if !ok {
		return nil, false
	}
	if exact, ok := current.cursorRow(preferred); ok {
		return exact, true
	}
	if current.rightRow != nil && current.rightRow.kind == rowCode {
		return current.rightRow, true
	}
	if current.leftRow != nil && current.leftRow.kind == rowCode {
		return current.leftRow, true
	}
	return nil, false
}

func (projection sideBySideProjection) paneRowAtOrAbove(source *row, target pane) (*row, bool) {
	current := projection.sideBySideRowBySourceRow[source]
	for current != nil {
		if candidate, ok := current.cursorRow(target); ok {
			return candidate, true
		}
		current = current.previous
	}
	return nil, false
}

func (projection sideBySideProjection) paneChangeAtOrBelow(source *row, target pane) (*row, bool) {
	current := projection.sideBySideRowBySourceRow[source]
	for current != nil {
		if candidate, ok := current.cursorRow(target); ok {
			switch target {
			case paneLeft:
				if candidate.line.Kind == review.LineRemoved {
					return candidate, true
				}
			case paneRight:
				if candidate.line.Kind == review.LineAdded {
					return candidate, true
				}
			}
		}
		current = current.next
	}
	return nil, false
}

func (p pane) other() pane {
	if p == paneLeft {
		return paneRight
	}
	return paneLeft
}
