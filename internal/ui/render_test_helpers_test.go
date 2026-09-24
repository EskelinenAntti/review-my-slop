package ui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

func (m Model) viewLabel() string {
	review := &m.review
	progress := ""
	if review.viewport.Top > 0 {
		progress = fmt.Sprintf(" (%d%%)", review.view.ViewportProgress(review.viewport))
	}
	if branch := m.currentBranch(); branch != "" {
		return "branch changes from " + branch + progress
	}
	return "local changes" + progress
}

func (m Model) renderStatus() string {
	search := &m.search
	status := "j/k/h/l move  c comment  ? help  q quit"
	if m.mode == modeSearch {
		status = "/" + string(search.query) + editorCursorStyle.Render(" ")
		if search.miss {
			status += errorStyle.Render("  no matches")
		}
	} else if m.review.selection != nil {
		status = "visual selection  j/k extend  c comment  Esc cancel"
	}
	return m.renderFooter(mutedStyle.Render(status))
}

func renderKeyBindings(bindings []keyBinding) []string {
	width := 0
	for _, binding := range bindings {
		width = max(width, lipgloss.Width(binding.keys))
	}
	var lines []string
	for _, binding := range bindings {
		lines = append(lines, binding.keys+strings.Repeat(" ", width-lipgloss.Width(binding.keys))+"  "+binding.description)
	}
	return lines
}
