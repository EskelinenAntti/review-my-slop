package diffscreen

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/ui/internal/frame"
)

// diffRenderer keeps presentation state local to a single render call.
type diffRenderer struct {
	view                  *diffView
	viewport              diffViewport
	cursor                diffCursor
	selection             *diffSelection
	leftWidth, rightWidth int
}

func (v *diffView) renderRows(
	viewport diffViewport,
	cursor diffCursor,
	selection *diffSelection,
) []string {
	viewport = v.clampViewport(viewport)
	renderer := diffRenderer{
		view:      v,
		viewport:  viewport,
		cursor:    cursor,
		selection: selection,
	}
	if v.split {
		renderer.leftWidth, renderer.rightWidth = splitPaneWidths(
			viewport.width,
		)
	}
	lines := make([]string, 0, viewport.height)
	if v.hasStickyHeader(viewport.top, viewport.height) {
		current := v.rows[viewport.top]
		lines = append(
			lines,
			renderFileRow(
				v.patch.Files[current.file].DisplayPath,
				viewport.width,
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
	for len(lines) < viewport.height {
		lines = append(lines, "")
	}
	return lines
}

func (r diffRenderer) renderUnifiedRow(current entry, y int) string {
	width := frame.ScreenWidth(r.viewport.width)
	switch current.kind {
	case fileRow:
		return renderFileRow(current.text, width)
	case metadataRow:
		return metadataStyle.Render("  " + current.text)
	case hunkRow:
		return hunkStyle.Render(current.text)
	case lineRow:
		return r.renderUnifiedLine(current, y, width)
	}
	return ""
}

func (r diffRenderer) renderUnifiedLine(
	current entry,
	y, width int,
) string {
	hunk := r.view.patch.Files[current.file].Hunks[current.hunk]
	line := hunk.Lines[current.rightLine]
	prefix := lineMarker(line.Kind)
	gutter := fmt.Sprintf(
		"%5s %5s %s ",
		number(line.OldNumber),
		number(line.NewNumber),
		prefix,
	)
	return r.renderCodeLine(
		gutter,
		current.text,
		width,
		line,
		diffCursor{row: y, pane: r.cursor.pane},
	)
}

func renderFileRow(path string, width int) string {
	return fileStyle.Width(frame.ScreenWidth(width)).Render(path)
}

func (r diffRenderer) renderSplitRow(current entry, y int) string {
	left := r.renderPane(current, y, left, r.leftWidth)
	right := r.renderPane(current, y, right, r.rightWidth)
	return left + " │ " + right
}

func splitPaneWidths(width int) (left, right int) {
	left = max(20, (width-3)/2)
	return left, max(20, width-3-left)
}

func (r diffRenderer) renderPane(
	current entry,
	y int,
	pane diffPane,
	width int,
) string {
	index := r.view.lineIndex(current, pane)
	if index < 0 {
		return strings.Repeat(" ", width)
	}
	line := r.view.patch.Files[current.file].Hunks[current.hunk].Lines[index]
	text := current.right
	numberValue := line.NewNumber
	if pane == left {
		text, numberValue = current.left, line.OldNumber
	}
	prefix := lineMarker(line.Kind) + " "
	gutter := fmt.Sprintf("%5s ", number(numberValue))
	return r.renderCodeLine(
		gutter,
		prefix+text,
		width,
		line,
		diffCursor{row: y, pane: pane},
	)
}

func (r diffRenderer) renderCodeLine(
	gutter, text string,
	width int,
	line patch.Line,
	candidate diffCursor,
) string {
	value := gutter + fitANSIWindow(
		text,
		r.viewport.leftColumn,
		width-lipgloss.Width(gutter),
	)
	style, colors := r.lineStyle(line.Kind, candidate)
	return renderStyledRow(style, value, width, colors)
}

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
	style, colors := lineStyle(kind, r.view.styles), preserveForeground
	if r.selection.contains(candidate) {
		style, colors = r.view.styles.selection, discardForeground
	}
	if r.cursor == candidate {
		style, colors = cursorStyle, discardForeground
	}
	return style, colors
}

func lineStyle(kind patch.LineKind, styles lineStyles) lipgloss.Style {
	switch kind {
	case patch.Addition:
		return styles.added
	case patch.Deletion:
		return styles.removed
	default:
		return contextStyle
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

func number(value patch.LineNumber) string {
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
