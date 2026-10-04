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
	v.keepVisible()
}
func (v *View) Resize(width, height int) {
	v.width, v.height = width, height
	v.keepVisible()
}
func (v *View) Move(delta int) {
	delta = max(-v.row, min(delta, max(0, len(v.items)-1)-v.row))
	v.row += delta
	v.keepVisible()
}
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
	row += v.startRow(height)
	if row >= len(v.items) {
		return false
	}
	v.row = row
	return true
}

func (v *View) startRow(height int) int {
	return min(max(0, v.top), max(0, len(v.items)-height))
}

func (v *View) keepVisible() {
	height := frame.BodyHeight(v.height)
	v.top = min(v.top, v.row)
	v.top = max(v.top, v.row-height+1)
	v.top = v.startRow(height)
}

// Render composes the screen, showing a non-nil error in place of its hints.
func (v *View) Render(err error) string {
	header := titleStyle.Render("comments") + "  " + mutedStyle.Render(fmt.Sprintf("%d pending", len(v.items)))
	height := frame.BodyHeight(v.height)
	body := make([]string, 0, height)
	if len(v.items) == 0 {
		body = make([]string, height)
		body[min(1, height-1)] = mutedStyle.Render("No pending comments.")
	} else {
		start := v.startRow(height)
		end := min(len(v.items), start+height)
		for index := start; index < end; index++ {
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
			line := ansi.Truncate(fmt.Sprintf("%s%s  %s", prefix, location, text), max(20, v.width), "")
			body = append(body, style.Width(max(20, v.width)).Render(line))
		}
	}
	footer := mutedStyle.Render("j/k move  Enter/e edit  D delete  Esc/q return")
	if err != nil {
		message := strings.ReplaceAll(err.Error(), "\n", " ")
		footer = errorStyle.Render(ansi.Truncate(message, max(20, v.width), ""))
	}
	return frame.Render(header, body, footer, v.height)
}

var (
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Cyan)
	mutedStyle = lipgloss.NewStyle().Faint(true)
	errorStyle = lipgloss.NewStyle().Foreground(lipgloss.Red).Bold(true)
)
