// Package commentscreen presents a selectable list of pending comments.
package commentscreen

import (
	"fmt"
	"slices"

	"github.com/eskelinenantti/review-my-slop/internal/comments"
	"github.com/eskelinenantti/review-my-slop/internal/ui"
	"github.com/eskelinenantti/review-my-slop/internal/ui/internal/frame"
	"github.com/eskelinenantti/review-my-slop/internal/ui/internal/scroll"
)

// View owns list focus, scrolling, and comment presentation.
type View struct {
	items         []comments.Comment
	pendingLabel  string
	row           int
	top           int
	width, height int
}

// New creates a comment list view with default terminal dimensions.
func New(items []comments.Comment) *View {
	v := &View{
		width:  ui.DefaultScreenWidth,
		height: ui.DefaultScreenHeight,
	}
	v.Update(items)
	return v
}

// Update preserves focus by comment ID, falling back to the nearest list index
// when the selected comment disappears. Empty lists are valid.
func (v *View) Update(items []comments.Comment) {
	selectedID := ""
	if selected, ok := v.Selected(); ok {
		selectedID = selected.ID
	}
	v.items = slices.Clone(items)
	v.pendingLabel = fmt.Sprintf("%d pending", len(v.items))
	v.row = min(v.row, max(0, len(v.items)-1))
	if selectedID != "" {
		index := slices.IndexFunc(v.items, func(item comments.Comment) bool {
			return item.ID == selectedID
		})
		if index >= 0 {
			v.row = index
		}
	}
	v.keepVisible()
}

// Resize updates terminal dimensions and keeps the focused comment visible.
func (v *View) Resize(width, height int) {
	v.width, v.height = width, height
	v.keepVisible()
}

// Move shifts focus by delta comments within the list bounds.
func (v *View) Move(delta int) {
	v.row = scroll.Offset(v.row, delta, max(0, len(v.items)-1))
	v.keepVisible()
}

// Selected returns the focused comment, or false when the list is empty.
func (v *View) Selected() (comments.Comment, bool) {
	index, ok := v.SelectedIndex()
	if !ok {
		return comments.Comment{}, false
	}
	return v.items[index], true
}

// SelectedIndex returns the focused comment's list index.
func (v *View) SelectedIndex() (int, bool) {
	if len(v.items) == 0 {
		return 0, false
	}
	return v.row, true
}

// Click focuses a visible comment at zero-based terminal coordinates.
func (v *View) Click(x, y int) bool {
	height := frame.BodyHeight(v.height)
	row, ok := frame.BodyRow(x, y, v.width, v.height)
	if !ok {
		return false
	}
	row += v.firstVisibleItem(height)
	if row >= len(v.items) {
		return false
	}
	v.row = row
	return true
}

func (v *View) firstVisibleItem(height int) int {
	return min(max(0, v.top), max(0, len(v.items)-height))
}

func (v *View) keepVisible() {
	height := frame.BodyHeight(v.height)
	v.top = min(v.top, v.row)
	v.top = max(v.top, v.row-height+1)
	v.top = v.firstVisibleItem(height)
}
