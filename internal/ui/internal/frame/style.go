package frame

import "charm.land/lipgloss/v2"

var (
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Cyan)
	mutedStyle = lipgloss.NewStyle().Faint(true)
	errorStyle = lipgloss.NewStyle().Foreground(lipgloss.Red).Bold(true)
)

// Title renders a screen title using the shared title style.
func Title(value string) string { return titleStyle.Render(value) }

// Muted renders secondary screen text using the shared muted style.
func Muted(value string) string { return mutedStyle.Render(value) }

// Error renders status text using the shared error style.
func Error(value string) string { return errorStyle.Render(value) }
