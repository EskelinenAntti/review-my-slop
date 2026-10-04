// Package helpscreen renders keyboard binding descriptions.
package helpscreen

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/eskelinenantti/review-my-slop/internal/ui/internal/frame"
)

type Binding struct {
	Keys        string
	Description string
}

// Render formats bindings with aligned descriptions and keeps the closing hint
// below the available body. The supplied bindings determine the help content.
func Render(bindings []Binding, width, height int) string {
	column := 0
	for _, binding := range bindings {
		column = max(column, lipgloss.Width(binding.Keys))
	}
	body := []string{""}
	for _, binding := range bindings {
		line := binding.Keys + strings.Repeat(" ", column-lipgloss.Width(binding.Keys)) + "  " + binding.Description
		body = append(body, ansi.Truncate(line, max(20, width), ""))
	}
	return frame.Render(
		lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Cyan).Render("review-my-slop help"),
		body,
		lipgloss.NewStyle().Faint(true).Render("? or Esc closes help"),
		height,
	)
}
