// Package commentscreen presents a selectable list of pending comments.
package commentscreen

import (
	"fmt"
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

// New constructs a comment list screen.
func New(items []comments.Comment) *View {
	v := &View{width: defaultWidth, height: defaultHeight}
	v.Update(items)
	return v
}

// Update preserves focus by comment ID, falling back to the nearest list index
// when the selected comment disappears. Empty lists are valid.
func (v *View) Update(items []comments.Comment) {
	selected, ok := v.Selected()
	v.items = append([]comments.Comment(nil), items...)
	v.row = min(v.row, max(zero, len(items)-one))
	if ok && selected.ID != "" {
		for index, item := range items {
			if item.ID == selected.ID {
				v.row = index
				break
			}
		}
	}
	v.keepVisible()
}

// Resize sets the terminal dimensions used to render the screen.
func (v *View) Resize(width, height int) {
	v.width, v.height = width, height
	v.keepVisible()
}

// Move moves the active selection within the comment list.
func (v *View) Move(initialDelta int) {
	delta := initialDelta

	delta = max(-v.row, min(delta, max(zero, len(v.items)-one)-v.row))
	v.row += delta
	v.keepVisible()
}

// Selected returns the currently selected comment.
func (v *View) Selected() (comments.Comment, bool) {
	if len(v.items) == zero {
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
	row += v.startRow(height)
	if row >= len(v.items) {
		return false
	}
	v.row = row
	return true
}

func (v *View) startRow(height int) int {
	return min(max(zero, v.top), max(zero, len(v.items)-height))
}

func (v *View) keepVisible() {
	height := frame.BodyHeight(v.height)
	v.top = min(v.top, v.row)
	v.top = max(v.top, v.row-height+one)
	v.top = v.startRow(height)
}

// Render composes the screen, showing a non-nil error in place of its hints.
func (v *View) Render(err error) string {
	header := titleStyle.Render("comments") + "  " +
		mutedStyle.Render(fmt.Sprintf("%d pending", len(v.items)))
	height := frame.BodyHeight(v.height)
	body := make([]string, zero, height)
	if len(v.items) == zero {
		body = make([]string, height)
		body[min(one, height-one)] = mutedStyle.Render("No pending comments.")
	} else {
		start := v.startRow(height)
		end := min(len(v.items), start+height)
		for index := start; index < end; index++ {
			body = append(body, v.renderComment(index))
		}
	}
	footer := mutedStyle.Render(
		"j/k move  Enter/e edit  D delete  Esc/q return",
	)
	if err != nil {
		message := strings.ReplaceAll(err.Error(), "\n", " ")
		footer = errorStyle.Render(
			ansi.Truncate(message, max(minimumWidth, v.width), ""),
		)
	}
	return frame.Render(header, body, footer, v.height)
}

var (
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Cyan)
	mutedStyle = lipgloss.NewStyle().Faint(true)
	errorStyle = lipgloss.NewStyle().Foreground(lipgloss.Red).Bold(true)
)

func (v *View) renderComment(index int) string {
	comment := v.items[index]
	prefix, style := "  ", lipgloss.NewStyle()
	if index == v.row {
		prefix, style = "> ", lipgloss.NewStyle().Reverse(true)
	}
	location := comment.Anchor.FilePath
	if comment.Anchor.NewStart > zero {
		location += fmt.Sprintf(":%d", comment.Anchor.NewStart)
	} else if comment.Anchor.OldStart > zero {
		location += fmt.Sprintf(":%d", comment.Anchor.OldStart)
	}
	text := strings.ReplaceAll(strings.TrimSpace(comment.Body), "\n", " ")
	line := ansi.Truncate(
		fmt.Sprintf("%s%s  %s", prefix, location, text),
		max(minimumWidth, v.width),
		"",
	)
	return style.Width(max(minimumWidth, v.width)).Render(line)
}
