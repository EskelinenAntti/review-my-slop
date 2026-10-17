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
	header := titleStyle.Render("review-my-slop help")
	footer := mutedStyle.Render("? or Esc closes help")
	return renderFrame(header, body, footer, height)
}
