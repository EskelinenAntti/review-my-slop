package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// Binding describes an application key binding for the help screen.
type Binding struct {
	Keys        string
	Description string
}

// RenderHelp formats bindings within the available terminal dimensions.
func RenderHelp(
	bindings []Binding,
	width, height int,
) string {
	column := 0
	for _, binding := range bindings {
		column = max(column, lipgloss.Width(binding.Keys))
	}
	body := []string{""}
	for _, binding := range bindings {
		line := binding.Keys + strings.Repeat(" ", column-lipgloss.Width(
			binding.Keys,
		)) +
			"  " + binding.Description
		body = append(body, ansi.Truncate(line, max(20, width), ""))
	}
	return renderHelpContent(body, height)
}

func renderHelpContent(body []string, height int) string {
	bodyHeight := max(1, height-3)
	lines := make([]string, 0, bodyHeight+3)
	lines = append(lines, lipgloss.NewStyle().Bold(
		true,
	).Foreground(lipgloss.Cyan).Render(
		"review-my-slop help",
	))
	lines = append(lines, body[:min(len(body), bodyHeight)]...)
	for len(lines) < bodyHeight+1 {
		lines = append(lines, "")
	}
	lines = append(lines, lipgloss.NewStyle().Faint(
		true,
	).Render("? or Esc closes help"), "")
	return strings.Join(lines, "\n")
}
