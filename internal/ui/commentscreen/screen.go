// Package commentscreen presents a selectable list of pending comments.
package commentscreen

import (
	"fmt"
	"slices"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
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
	selectedID := ""
	if index, ok := v.SelectedIndex(); ok {
		selectedID = v.items[index].ID
	}
	v.items = slices.Clone(items)
	v.row = min(v.row, max(0, len(v.items)-1))
	if selectedID != "" {
		for index, item := range v.items {
			if item.ID == selectedID {
				v.row = index
				break
			}
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
	delta = max(-v.row, min(delta, max(0, len(v.items)-1)-v.row))
	v.row += delta
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
	row += v.firstVisibleRow(height)
	if row >= len(v.items) {
		return false
	}
	v.row = row
	return true
}

func (v *View) firstVisibleRow(height int) int {
	return min(max(0, v.top), max(0, len(v.items)-height))
}

func (v *View) keepVisible() {
	height := frame.BodyHeight(v.height)
	v.top = min(v.top, v.row)
	v.top = max(v.top, v.row-height+1)
	v.top = v.firstVisibleRow(height)
}

// Render composes the screen, showing a non-nil error in place of its hints.
func (v *View) Render(err error) string {
	pending := frame.Muted(fmt.Sprintf("%d pending", len(v.items)))
	header := frame.Title("comments") + "  " + pending
	height := frame.BodyHeight(v.height)
	var body []string
	if len(v.items) == 0 {
		body = frame.EmptyBody(v.height, "No pending comments.")
	} else {
		body = make([]string, 0, height)
		start := v.firstVisibleRow(height)
		end := min(len(v.items), start+height)
		for index := start; index < end; index++ {
			body = append(body, v.renderRow(index))
		}
	}
	footer := frame.Muted(
		"j/k move  Enter/e edit  D delete  Esc/q return",
	)
	if err != nil {
		message := frame.SingleLine(err.Error())
		footer = frame.Error(ansi.Truncate(message, max(20, v.width), ""))
	}
	return frame.Render(header, body, footer, v.height)
}

func (v *View) renderRow(index int) string {
	comment := v.items[index]
	prefix, style := "  ", lipgloss.NewStyle()
	if index == v.row {
		prefix, style = "> ", lipgloss.NewStyle().Reverse(true)
	}
	location := comment.Anchor.FilePath
	if comment.Anchor.NewStart > 0 {
		location += fmt.Sprintf(":%d", comment.Anchor.NewStart)
	} else if comment.Anchor.OldStart > 0 {
		location += fmt.Sprintf(":%d", comment.Anchor.OldStart)
	}
	text := strings.ReplaceAll(strings.TrimSpace(comment.Body), "\n", " ")
	line := ansi.Truncate(
		fmt.Sprintf("%s%s  %s", prefix, location, text),
		max(20, v.width),
		"",
	)
	return style.Width(max(20, v.width)).Render(line)
}
