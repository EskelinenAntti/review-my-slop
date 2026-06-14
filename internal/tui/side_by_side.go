package tui

import "github.com/eskelinenantti/review-my-slop/internal/review"

type pane uint8

const (
	paneLeft pane = iota
	paneRight
)

const minimumSideBySideWidth = 100

type visualRow struct {
	source int
	left   int
	right  int
}

type visualLayout struct {
	rows           []row
	visual         []visualRow
	sourceToVisual []int
	sideBySide     bool
}

func newVisualLayout(rows []row, sideBySide bool) visualLayout {
	layout := visualLayout{
		rows:           rows,
		sourceToVisual: make([]int, len(rows)),
		sideBySide:     sideBySide,
	}
	if !sideBySide {
		layout.visual = make([]visualRow, len(rows))
		for index := range rows {
			layout.visual[index] = visualRow{source: index, left: index, right: index}
			layout.sourceToVisual[index] = index
		}
		return layout
	}

	for index := 0; index < len(rows); {
		current := rows[index]
		if current.kind != rowCode {
			layout.appendRow(visualRow{source: index, left: index, right: index})
			index++
			continue
		}
		switch current.line.Kind {
		case review.LineContext:
			layout.appendRow(visualRow{source: index, left: index, right: index})
			index++
		case review.LineAdded:
			layout.appendRow(visualRow{source: index, left: -1, right: index})
			index++
		case review.LineRemoved:
			removedEnd := layout.changeBlockEnd(index, review.LineRemoved)
			addedStart := removedEnd
			addedEnd := addedStart
			if addedStart < len(rows) && layout.sameHunkKind(addedStart, current, review.LineAdded) {
				addedEnd = layout.changeBlockEnd(addedStart, review.LineAdded)
			}
			removedCount := removedEnd - index
			addedCount := addedEnd - addedStart
			for offset := 0; offset < max(removedCount, addedCount); offset++ {
				left, right := -1, -1
				if offset < removedCount {
					left = index + offset
				}
				if offset < addedCount {
					right = addedStart + offset
				}
				source := left
				if source < 0 {
					source = right
				}
				layout.appendRow(visualRow{source: source, left: left, right: right})
			}
			index = addedEnd
		}
	}
	return layout
}

func (layout *visualLayout) appendRow(current visualRow) {
	position := len(layout.visual)
	layout.visual = append(layout.visual, current)
	if current.left >= 0 {
		layout.sourceToVisual[current.left] = position
	}
	if current.right >= 0 {
		layout.sourceToVisual[current.right] = position
	}
}

func (layout visualLayout) changeBlockEnd(start int, kind review.LineKind) int {
	current := layout.rows[start]
	end := start
	for end < len(layout.rows) && layout.sameHunkKind(end, current, kind) {
		end++
	}
	return end
}

func (layout visualLayout) sameHunkKind(index int, current row, kind review.LineKind) bool {
	return index >= 0 &&
		index < len(layout.rows) &&
		layout.rows[index].kind == rowCode &&
		layout.rows[index].fileIndex == current.fileIndex &&
		layout.rows[index].hunkIndex == current.hunkIndex &&
		layout.rows[index].line.Kind == kind
}

func (layout visualLayout) len() int {
	return len(layout.visual)
}

func (layout visualLayout) row(position int) visualRow {
	if len(layout.visual) == 0 {
		return visualRow{source: -1, left: -1, right: -1}
	}
	position = max(0, min(position, len(layout.visual)-1))
	return layout.visual[position]
}

func (layout visualLayout) position(source int) int {
	if len(layout.sourceToVisual) == 0 {
		return 0
	}
	source = max(0, min(source, len(layout.sourceToVisual)-1))
	return layout.sourceToVisual[source]
}

func (layout visualLayout) paneIndex(position int, target pane) int {
	current := layout.row(position)
	index := current.right
	if target == paneLeft {
		index = current.left
	}
	if index >= 0 && index < len(layout.rows) && layout.rows[index].kind == rowCode {
		return index
	}
	return -1
}

func (layout visualLayout) cursorIndex(position int, preferred pane) int {
	if exact := layout.paneIndex(position, preferred); exact >= 0 {
		return exact
	}
	current := layout.row(position)
	if current.right >= 0 && layout.rows[current.right].kind == rowCode {
		return current.right
	}
	if current.left >= 0 && layout.rows[current.left].kind == rowCode {
		return current.left
	}
	return -1
}

func (layout visualLayout) paneIndexAtOrAbove(position int, target pane) int {
	position = min(position, layout.len()-1)
	for current := position; current >= 0; current-- {
		if index := layout.paneIndex(current, target); index >= 0 {
			return index
		}
	}
	return -1
}

func (layout visualLayout) navigable(position, direction int, target pane) (int, int) {
	for current := position + direction; current >= 0 && current < layout.len(); current += direction {
		index := layout.cursorIndex(current, target)
		if layout.sideBySide {
			index = layout.paneIndex(current, target)
		}
		if index >= 0 {
			return current, index
		}
	}
	return -1, -1
}

func (layout visualLayout) codeNearBetween(position, direction, first, last int, target pane) (int, int) {
	for current := position; current >= first && current <= last; current += direction {
		index := layout.cursorIndex(current, target)
		if layout.sideBySide {
			index = layout.paneIndex(current, target)
		}
		if index >= 0 {
			return current, index
		}
	}
	return -1, -1
}

func (p pane) other() pane {
	if p == paneLeft {
		return paneRight
	}
	return paneLeft
}
