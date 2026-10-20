package diffscreen

import (
	"image/color"

	"charm.land/lipgloss/v2"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

var (
	fileStyle     = lipgloss.NewStyle().Bold(true)
	metadataStyle = lipgloss.NewStyle().Faint(true)
	hunkStyle     = lipgloss.NewStyle().Foreground(lipgloss.Magenta)
	contextStyle  = lipgloss.NewStyle()
	addedStyle    = lipgloss.NewStyle().Foreground(lipgloss.Green)
	removedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Red)
	cursorStyle   = lipgloss.NewStyle().Reverse(true)
)

func lineMarker(kind patch.LineKind) string {
	switch kind {
	case patch.Addition:
		return addedStyle.Render(kind.DiffPrefix())
	case patch.Deletion:
		return removedStyle.Render(kind.DiffPrefix())
	default:
		return kind.DiffPrefix()
	}
}

func (r diffRenderer) lineStyle(
	kind patch.LineKind,
	candidate diffCursor,
) (lipgloss.Style, foregroundPolicy) {
	if r.cursor == candidate {
		return cursorStyle, discardForeground
	}
	if r.selection.contains(candidate) {
		return r.view.styles.selection, discardForeground
	}
	switch kind {
	case patch.Addition:
		return r.view.styles.added, preserveForeground
	case patch.Deletion:
		return r.view.styles.removed, preserveForeground
	default:
		return contextStyle, preserveForeground
	}
}

type lineStyles struct {
	added, removed, selection lipgloss.Style
}

func newLineStyles(dark bool) lineStyles {
	return lineStyles{
		added: backgroundStyle(
			dark,
			lipgloss.Color("#dafbe1"),
			lipgloss.Color("#1b3823"),
		),
		removed: backgroundStyle(
			dark,
			lipgloss.Color("#ffebe9"),
			lipgloss.Color("#402222"),
		),
		selection: backgroundStyle(
			dark,
			lipgloss.Color("#dbeafe"),
			lipgloss.Color("#1e3a5f"),
		),
	}
}

func backgroundStyle(
	dark bool,
	lightColor, darkColor color.Color,
) lipgloss.Style {
	return lipgloss.NewStyle().Background(
		lipgloss.LightDark(dark)(lightColor, darkColor),
	)
}
