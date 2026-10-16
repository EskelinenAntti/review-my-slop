package app

import (
	tea "charm.land/bubbletea/v2"
	"github.com/eskelinenantti/review-my-slop/internal/ui/helpscreen"
)

func (m model) View() tea.View {
	if m.quitting {
		return tea.NewView("")
	}
	result := tea.NewView(m.formatCurrentScreen())
	result.AltScreen = true
	result.ReportFocus = true
	result.MouseMode = tea.MouseModeCellMotion
	return result
}
func (m model) formatCurrentScreen() string {
	switch m.mode {
	case modeHelp:
		return m.formatHelpScreen()
	case modeComments:
		return m.commentView.FormatCommentsScreen(m.err)
	default:
		return m.diffView.FormatReviewScreen(m.err)
	}
}
func (m model) formatHelpScreen() string {
	bindings := []helpscreen.Binding{
		{Keys: "j/k, arrows", Description: "move"},
		{Keys: "h/l, left/right", Description: "scroll horizontally"},
		{Keys: "Mouse wheel", Description: "scroll content"},
		{Keys: "Click/drag", Description: "focus line/select range"},
		{Keys: "Ctrl-w h/l/w", Description: "switch side-by-side pane"},
		{Keys: "0/$", Description: "start/end of lines"},
		{Keys: "gg/G", Description: "first/last changed line"},
		{Keys: "zz/zt/zb", Description: "center/top/bottom current line"},
		{Keys: "Ctrl-d/Ctrl-u", Description: "half-page down/up"},
		{Keys: "/", Description: "findTextInChanges diff text"},
		{Keys: "n/N", Description: "next/previous findTextInChanges match"},
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

	return helpscreen.FormatHelpScreen(bindings, m.width, m.height)
}
