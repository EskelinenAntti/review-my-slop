package commentscreen

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/eskelinenantti/review-my-slop/internal/ui/internal/frame"
)

// Render composes the screen, showing a non-nil error in place of its hints.
func (v *View) Render(err error) string {
	pending := mutedStyle.Render(fmt.Sprintf("%d pending", len(v.items)))
	header := titleStyle.Render("comments") + "  " + pending
	height := frame.BodyHeight(v.height)
	body := make([]string, 0, height)
	if len(v.items) == 0 {
		body = make([]string, height)
		body[min(1, height-1)] = mutedStyle.Render("No pending comments.")
	} else {
		start := v.firstVisibleComment(height)
		end := min(len(v.items), start+height)
		for index := start; index < end; index++ {
			body = append(body, v.renderCommentRow(index))
		}
	}
	footer := mutedStyle.Render(
		"j/k move  Enter/e edit  D delete  Esc/q return",
	)
	if err != nil {
		message := strings.ReplaceAll(err.Error(), "\n", " ")
		footer = errorStyle.Render(ansi.Truncate(message, max(20, v.width), ""))
	}
	return frame.Render(header, body, footer, v.height)
}

func (v *View) renderCommentRow(index int) string {
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
