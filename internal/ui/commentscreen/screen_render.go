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
	width := frame.ScreenWidth(v.width)
	pending := frame.Muted(v.pendingLabel)
	header := frame.Title("comments") + "  " + pending
	height := frame.BodyHeight(v.height)
	var body []string
	if len(v.items) == 0 {
		body = frame.EmptyBody(v.height, "No pending comments.")
	} else {
		rowStyle := commentRowStyle.Width(width)
		selectedRowStyle := selectedCommentRowStyle.Width(width)
		body = make([]string, 0, height)
		start := v.firstVisibleItem(height)
		end := min(len(v.items), start+height)
		for index := start; index < end; index++ {
			body = append(
				body,
				v.renderRow(index, width, rowStyle, selectedRowStyle),
			)
		}
	}
	footer := frame.Muted(
		"j/k move  Enter/e edit  D delete  Esc/q return",
	)
	if err != nil {
		message := frame.SingleLine(err.Error())
		footer = frame.Error(ansi.Truncate(message, width, ""))
	}
	return frame.Render(header, body, footer, v.height)
}

func (v *View) renderRow(
	index, width int,
	rowStyle, selectedRowStyle lipgloss.Style,
) string {
	comment := v.items[index]
	prefix, style := "  ", rowStyle
	if index == v.row {
		prefix, style = "> ", selectedRowStyle
	}
	location := comment.Anchor.FilePath
	if comment.Anchor.NewStart > 0 {
		location += fmt.Sprintf(":%d", comment.Anchor.NewStart)
	} else if comment.Anchor.OldStart > 0 {
		location += fmt.Sprintf(":%d", comment.Anchor.OldStart)
	}
	text := frame.SingleLine(strings.TrimSpace(comment.Body))
	line := ansi.Truncate(
		fmt.Sprintf("%s%s  %s", prefix, location, text),
		width,
		"",
	)
	return style.Render(line)
}

var (
	commentRowStyle         = lipgloss.NewStyle()
	selectedCommentRowStyle = lipgloss.NewStyle().Reverse(true)
)
