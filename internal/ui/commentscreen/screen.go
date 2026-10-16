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

// NewCommentList creates a comment list with default terminal dimensions.
func NewCommentList(items []comments.Comment) *View {
	v := &View{width: 80, height: 30}
	v.ReplaceComments(items)
	return v
}

// ReplaceComments preserves focus by comment ID and falls back to a nearby
// comment when the selected comment disappears. Empty lists are valid.
func (v *View) ReplaceComments(items []comments.Comment) {
	selected, ok := v.SelectedComment()
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
	v.keepSelectedCommentVisible()
}

// Resize updates terminal dimensions and keeps the focused comment visible.
func (v *View) Resize(width, height int) {
	v.width, v.height = width, height
	v.keepSelectedCommentVisible()
}

// MoveCommentSelection shifts focus by delta within the list bounds.
func (v *View) MoveCommentSelection(delta int) {
	delta = max(-v.row, min(delta, max(0, len(v.items)-1)-v.row))
	v.row += delta
	v.keepSelectedCommentVisible()
}

// SelectedComment returns the focused comment, or false when the list is empty.
func (v *View) SelectedComment() (comments.Comment, bool) {
	if len(v.items) == 0 {
		return comments.Comment{}, false
	}
	return v.items[v.row], true
}

// Click focuses a visible comment at zero-based terminal coordinates.
func (v *View) Click(x, y int) bool {
	height := frame.ContentHeight(v.height)
	row, ok := frame.ContentRowAtPoint(x, y, v.width, v.height)
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

func (v *View) keepSelectedCommentVisible() {
	height := frame.ContentHeight(v.height)
	v.top = min(v.top, v.row)
	v.top = max(v.top, v.row-height+1)
	v.top = v.firstVisibleComment(height)
}

// FormatCommentsScreen composes the screen and shows errors in place of hints.
func (v *View) FormatCommentsScreen(err error) string {
	pending := mutedStyle.Render(fmt.Sprintf("%d pending", len(v.items)))
	header := titleStyle.Render("comments") + "  " + pending
	height := frame.ContentHeight(v.height)
	body := make([]string, 0, height)
	if len(v.items) == 0 {
		body = make([]string, height)
		body[min(1, height-1)] = mutedStyle.Render("No pending comments.")
	} else {
		start := v.firstVisibleComment(height)
		end := min(len(v.items), start+height)
		for index := start; index < end; index++ {
			body = append(body, v.formatCommentRow(index))
		}
	}
	footer := mutedStyle.Render(
		"j/k move  Enter/e edit  D delete  Esc/q return",
	)
	if err != nil {
		message := strings.ReplaceAll(err.Error(), "\n", " ")
		footer = errorStyle.Render(ansi.Truncate(message, max(20, v.width), ""))
	}
	return frame.ComposeScreen(header, body, footer, v.height)
}

func (v *View) formatCommentRow(index int) string {
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

var (
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Cyan)
	mutedStyle = lipgloss.NewStyle().Faint(true)
	errorStyle = lipgloss.NewStyle().Foreground(lipgloss.Red).Bold(true)
)
