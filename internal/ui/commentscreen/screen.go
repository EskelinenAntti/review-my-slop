// Package commentscreen presents a selectable list of pending comments.
package commentscreen

import (
	"github.com/eskelinenantti/review-my-slop/internal/comments"
	"github.com/eskelinenantti/review-my-slop/internal/ui/internal/frame"
)

// View owns list focus, scrolling, and comment presentation.
type View struct {
	items         []comments.Comment
	row           int
	top           int
	width, height int
}

// New creates a comment list view with default terminal dimensions.
func New(items []comments.Comment) *View {
	v := &View{width: 80, height: 30}
	v.Update(items)
	return v
}

// Update preserves focus by comment ID, falling back to the nearest list index
// when the selected comment disappears. Empty lists are valid.
func (v *View) Update(items []comments.Comment) {
	selected, ok := v.Selected()
	v.items = append([]comments.Comment(nil), items...)
	v.row = min(v.row, max(0, len(items)-1))
	if ok && selected.ID != "" {
		for index, item := range items {
			if item.ID == selected.ID {
				v.row = index
				break
			}
		}
	}
	v.keepFocusedCommentVisible()
}

// Resize updates terminal dimensions and keeps the focused comment visible.
func (v *View) Resize(width, height int) {
	v.width, v.height = width, height
	v.keepFocusedCommentVisible()
}

// Move shifts focus by delta comments within the list bounds.
func (v *View) Move(delta int) {
	delta = max(-v.row, min(delta, max(0, len(v.items)-1)-v.row))
	v.row += delta
	v.keepFocusedCommentVisible()
}

// Selected returns the focused comment, or false when the list is empty.
func (v *View) Selected() (comments.Comment, bool) {
	if len(v.items) == 0 {
		return comments.Comment{}, false
	}
	return v.items[v.row], true
}

// Click focuses a visible comment at zero-based terminal coordinates.
func (v *View) Click(x, y int) bool {
	height := frame.BodyHeight(v.height)
	row, ok := frame.BodyRow(x, y, v.width, v.height)
	if !ok {
		return false
	}
	row += v.firstVisibleComment(height)
	if row >= len(v.items) {
		return false
	}
	v.row = row
	return true
}

func (v *View) firstVisibleComment(height int) int {
	return min(max(0, v.top), max(0, len(v.items)-height))
}

func (v *View) keepFocusedCommentVisible() {
	height := frame.BodyHeight(v.height)
	v.top = min(v.top, v.row)
	v.top = max(v.top, v.row-height+1)
	v.top = v.firstVisibleComment(height)
}
