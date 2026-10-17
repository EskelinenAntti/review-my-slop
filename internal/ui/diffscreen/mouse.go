package diffscreen

import (
	"github.com/eskelinenantti/review-my-slop/internal/ui/internal/frame"
)

// BeginDrag focuses the code under a left-button press and clears any previous
// selection. Coordinates are zero-based terminal cells, including the header.
// Headers, metadata, empty panes, and cells outside the body are ignored.
func (v *View) BeginDrag(x, y int) bool {
	v.EndDrag()
	cursor, ok := func() (diffCursor, bool) {
		row, ok := frame.BodyRow(x, y, v.width, v.height)
		if !ok {
			return diffCursor{}, false
		}
		if func(v *diffView, inlineLocalTop1 int, viewportHeight int) bool {
			return viewportHeight > 1 && inlineLocalTop1 >= 0 && inlineLocalTop1 < len(v.rows) && v.rows[inlineLocalTop1].kind != fileRow
		}(v.view, v.viewport.top, v.viewport.Height) {
			row--
		}
		if row < 0 || row >= func(v *diffView, viewport diffViewport) int {
			height := viewport.Height
			if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != fileRow {
				height--
			}
			return max(1, height)
		}(v.view, v.viewport) {
			return diffCursor{}, false
		}
		pane := right
		if v.view.split {
			leftWidth, _ := func() (inlineLocalLeft2, inlineLocalRight1 int) {
				inlineLocalLeft2 = max(20, (v.viewport.Width-3)/2)
				return inlineLocalLeft2, max(20, v.viewport.Width-3-inlineLocalLeft2)
			}()
			switch {
			case x < leftWidth:
				pane = left
			case x < leftWidth+3:
				return diffCursor{}, false
			default:
				pane = right
			}
		}
		return func(v *diffView, y int) (diffCursor, bool) {
			cursor := diffCursor{row: y, pane: pane}
			return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
				var current entry = v.rows[cursor.row]
				if current.kind != lineRow {
					return -1
				}
				if !v.split || cursor.pane == right {
					return current.rightLine
				}
				return current.leftLine
			}() >= 0
		}(v.view, v.viewport.top+row)
	}()
	if !ok {
		return false
	}
	v.ClearSelection()
	v.cursor = cursor
	v.drag = &cursor
	return true
}

// DragTo extends a mouse selection within the starting hunk and pane. It does
// nothing without a code-line press, or when the pointer is over invalid code.
func (v *View) DragTo(x, y int) {
	if v.drag == nil {
		return
	}
	cursor, ok := func() (diffCursor, bool) {
		row, ok := frame.BodyRow(x, y, v.width, v.height)
		if !ok {
			return diffCursor{}, false
		}
		if func(v *diffView, inlineLocalTop2 int, viewportHeight int) bool {
			return viewportHeight > 1 && inlineLocalTop2 >= 0 && inlineLocalTop2 < len(v.rows) && v.rows[inlineLocalTop2].kind != fileRow
		}(v.view, v.viewport.top, v.viewport.Height) {
			row--
		}
		if row < 0 || row >= func(v *diffView, viewport diffViewport) int {
			height := viewport.Height
			if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != fileRow {
				height--
			}
			return max(1, height)
		}(v.view, v.viewport) {
			return diffCursor{}, false
		}
		pane := right
		if v.view.split {
			leftWidth, _ := func() (inlineLocalLeft1, inlineLocalRight2 int) {
				inlineLocalLeft1 = max(20, (v.viewport.Width-3)/2)
				return inlineLocalLeft1, max(20, v.viewport.Width-3-inlineLocalLeft1)
			}()
			switch {
			case x < leftWidth:
				pane = left
			case x < leftWidth+3:
				return diffCursor{}, false
			default:
				pane = right
			}
		}
		return func(v *diffView, y int) (diffCursor, bool) {
			cursor := diffCursor{row: y, pane: pane}
			return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
				var current entry = v.rows[cursor.row]
				if current.kind != lineRow {
					return -1
				}
				if !v.split || cursor.pane == right {
					return current.rightLine
				}
				return current.leftLine
			}() >= 0
		}(v.view, v.viewport.top+row)
	}()
	if !ok || cursor.pane != v.drag.pane {
		return
	}
	if cursor == *v.drag && v.selection == nil {
		return
	}
	selection, ok := func(v *diffView, selection diffSelection) (diffSelection, bool) {
		if !(selection.First.row >= 0 && selection.First.row < len(v.rows) && func() int {
			var (
				current entry    = v.rows[selection.First.row]
				pane    diffPane = selection.First.pane
			)
			if current.kind != lineRow {
				return -1
			}
			if !v.split || pane == right {
				return current.rightLine
			}
			return current.leftLine
		}() >= 0) || !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
			var current entry = v.rows[cursor.row]
			if current.kind != lineRow {
				return -1
			}
			if !v.split || cursor.pane == right {
				return current.rightLine
			}
			return current.leftLine
		}() >= 0) {
			return selection, false
		}
		first := v.rows[selection.First.row]
		last := v.rows[cursor.row]
		if first.file != last.file || first.hunk != last.hunk {
			return selection, false
		}
		selection.Last = cursor
		return selection, true
	}(v.view, func() diffSelection {
		var cursor diffCursor = *v.drag
		return diffSelection{First: cursor, Last: cursor}
	}())
	if !ok {
		return
	}
	v.cursor, v.selection = cursor, &selection
}

// EndDrag leaves a completed selection available for keyboard actions.
func (v *View) EndDrag() { v.drag = nil }
