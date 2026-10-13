package diffscreen

import (
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

// rowRenderer holds the viewport and interaction state shared by all rows
// in a single render pass.
type rowRenderer struct {
	projection *patchProjection
	viewport   patchViewport
	cursor     patchCursor
	selection  *patchRange
}

func (v *patchProjection) renderProjection(
	viewport patchViewport,
	cursor patchCursor,
	selection *patchRange,
) string {
	viewport = v.clampViewport(viewport)
	renderer := rowRenderer{
		projection: v,
		viewport:   viewport,
		cursor:     cursor,
		selection:  selection,
	}
	lines := make([]string, 0, viewport.Height)
	if v.showsStickyFileHeader(viewport.top, viewport.Height) {
		current := v.rows[viewport.top]
		lines = append(
			lines,
			renderFileHeaderRow(
				v.patch.Files[current.file].DisplayPath,
				viewport.Width,
			),
		)
	}
	end := min(len(v.rows), viewport.top+v.contentHeight(viewport))
	for y := viewport.top; y < end; y++ {
		current := v.rows[y]
		if v.split && current.kind == lineRow {
			lines = append(
				lines,
				renderer.renderSplitRow(current, y),
			)
		} else {
			lines = append(
				lines,
				renderer.renderUnifiedRow(current, y),
			)
		}
	}
	for len(lines) < viewport.Height {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func lineStyle(kind patch.LineKind, dark bool) lipgloss.Style {
	lightDark := lipgloss.LightDark(dark)
	switch kind {
	case patch.Addition:
		return lipgloss.NewStyle().Background(
			lightDark(lipgloss.Color("#dafbe1"), lipgloss.Color("#1b3823")),
		)
	case patch.Deletion:
		return lipgloss.NewStyle().Background(
			lightDark(lipgloss.Color("#ffebe9"), lipgloss.Color("#402222")),
		)
	default:
		return contextStyle
	}
}

func selectionRowStyle(dark bool) lipgloss.Style {
	return lipgloss.NewStyle().Background(
		lipgloss.LightDark(dark)(
			lipgloss.Color("#dbeafe"),
			lipgloss.Color("#1e3a5f"),
		),
	)
}

func formatLineNumber(value patch.LineNumber) string {
	if value == 0 {
		return ""
	}
	return strconv.Itoa(int(value))
}

var (
	fileStyle     = lipgloss.NewStyle().Bold(true)
	metadataStyle = lipgloss.NewStyle().Faint(true)
	hunkStyle     = lipgloss.NewStyle().Foreground(lipgloss.Magenta)
	contextStyle  = lipgloss.NewStyle()
	addedStyle    = lipgloss.NewStyle().Foreground(lipgloss.Green)
	removedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Red)
	cursorStyle   = lipgloss.NewStyle().Reverse(true)
)
