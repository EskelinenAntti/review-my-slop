package app

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/eskelinenantti/review-my-slop/internal/ui/helpscreen"
)

func (m model) View() tea.View {
	if m.quitting {
		return tea.NewView("")
	}
	result := tea.NewView(m.render())
	result.AltScreen = true
	result.ReportFocus = true
	return result
}
func (m model) render() string {
	var rendered string
	switch m.mode {
	case modeHelp:
		return m.renderHelp()
	case modeComments:
		rendered = m.commentList.Render()
	default:
		rendered = m.diff.Render()
	}
	if m.err != nil {
		lines := strings.Split(rendered, "\n")
		footer := len(lines) - 2
		message := lipgloss.NewStyle().Foreground(lipgloss.Red).Bold(true).Render(m.err.Error())
		if m.mode == modeComments {
			lines[footer] = message
		} else {
			// Diff footers have a right-aligned label separated by padding.
			// Preserve that label while replacing the application status.
			plain := ansi.Strip(lines[footer])
			label := ""
			index := max(strings.LastIndex(plain, "local changes"), strings.LastIndex(plain, "branch changes from "))
			if index >= 0 {
				label = plain[index:]
			}
			right := lipgloss.NewStyle().Faint(true).Render(label)
			width := max(20, m.width)
			message = ansi.Truncate(message, max(0, width-lipgloss.Width(right)-1), "")
			lines[footer] = message + strings.Repeat(" ", max(1, width-lipgloss.Width(message)-lipgloss.Width(right))) + right
		}
		rendered = strings.Join(lines, "\n")
	}
	return rendered
}
func (m model) renderHelp() string {
	bindings := []helpscreen.Binding{
		{Keys: "j/k, arrows", Description: "move"},
		{Keys: "h/l, left/right", Description: "scroll horizontally"},
		{Keys: "Ctrl-w h/l/w", Description: "switch side-by-side pane"},
		{Keys: "0/$", Description: "start/end of lines"},
		{Keys: "gg/G", Description: "first/last changed line"},
		{Keys: "zz/zt/zb", Description: "center/top/bottom current line"},
		{Keys: "Ctrl-d/Ctrl-u", Description: "half-page down/up"},
		{Keys: "/", Description: "search diff text"},
		{Keys: "n/N", Description: "next/previous search match"},
		{Keys: "]f/[f", Description: "next/previous file"},
		{Keys: "v", Description: "select a line range"},
		{Keys: "c", Description: "comment on selection/current line"},
		{Keys: "e", Description: "open current line in $EDITOR"},
		{Keys: "C", Description: "view comments"},
		{Keys: "R", Description: "refresh diff"},
		{Keys: "Tab", Description: "toggle local/branch changes"},
		{Keys: "t", Description: "toggle unified/side-by-side"},
		{Keys: "q", Description: "quit"},
	}

	return helpscreen.Render(bindings, m.width, m.height)
}
