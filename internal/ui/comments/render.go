package comments

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func (m *Model) Render() string {
	header := titleStyle.Render("comments") + "  " + mutedStyle.Render(fmt.Sprintf("%d pending", len(m.items)))
	height := max(1, m.height-3)
	body := make([]string, 0, height)
	if len(m.items) == 0 {
		body = make([]string, height)
		body[min(1, height-1)] = mutedStyle.Render("No pending comments.")
	} else {
		start := min(max(0, m.row-height+1), max(0, len(m.items)-height))
		end := min(len(m.items), start+height)
		for index := start; index < end; index++ {
			comment := m.items[index]
			prefix, style := "  ", screenContextStyle
			if index == m.row {
				prefix, style = "> ", screenCursorStyle
			}
			location := comment.Anchor.FilePath
			if comment.Anchor.NewStart > 0 {
				location += fmt.Sprintf(":%d", comment.Anchor.NewStart)
			} else if comment.Anchor.OldStart > 0 {
				location += fmt.Sprintf(":%d", comment.Anchor.OldStart)
			}
			text := strings.ReplaceAll(strings.TrimSpace(comment.Body), "\n", " ")
			line := ansi.Truncate(fmt.Sprintf("%s%s  %s", prefix, location, text), max(20, m.width), "")
			body = append(body, style.Width(max(20, m.width)).Render(line))
		}
	}
	footer := mutedStyle.Render("j/k move  Enter/e edit  D delete  Esc/q return")
	if m.err != nil {
		footer = errorStyle.Render(m.err.Error())
	}
	return renderScreen(m.height, header, body, footer)
}

func renderScreen(height int, header string, body []string, footer string) string {
	bodyHeight := max(1, height-3)
	if len(body) > bodyHeight {
		body = body[:bodyHeight]
	}
	for len(body) < bodyHeight {
		body = append(body, "")
	}
	lines := make([]string, 0, bodyHeight+3)
	lines = append(lines, header)
	lines = append(lines, body...)
	lines = append(lines, footer, "")
	return strings.Join(lines, "\n")
}

var (
	titleStyle         = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Cyan)
	screenContextStyle = lipgloss.NewStyle()
	screenCursorStyle  = lipgloss.NewStyle().Reverse(true)
	mutedStyle         = lipgloss.NewStyle().Faint(true)
	errorStyle         = lipgloss.NewStyle().Foreground(lipgloss.Red).Bold(true)
)
