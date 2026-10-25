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

type diffRenderer struct {
	*diffView
	viewport  diffViewport
	cursor    diffCursor
	selection *diffSelection
}

func (v *diffView) render(
	viewport diffViewport,
	cursor diffCursor,
	selection *diffSelection,
) string {
	viewport = v.clampViewport(viewport)
	renderer := diffRenderer{
		diffView:  v,
		viewport:  viewport,
		cursor:    cursor,
		selection: selection,
	}
	leftWidth, rightWidth := 0, 0
	if v.split {
		leftWidth, rightWidth = splitPaneWidths(viewport.width)
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
	start, end := v.visibleRows(viewport)
	for y := start; y < end; y++ {
		current := v.rows[y]
		if v.split && current.kind == lineRow {
			lines = append(
				lines,
				renderer.renderSplitRow(current, y, leftWidth, rightWidth),
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
	return strings.Join(lines, "\n")
}

func (v diffRenderer) renderUnifiedRow(
	current projectedRow,
	y int,
) string {
	width := frame.ScreenWidth(v.viewport.width)
	switch current.kind {
	case fileRow:
		return renderFileRow(current.text, width)
	case metadataRow:
		return metadataStyle.Render("  " + current.text)
	case hunkRow:
		return hunkStyle.Render(current.text)
	case lineRow:
		return v.renderUnifiedLine(current, y, width)
	}
	return ""
}

func (v diffRenderer) renderUnifiedLine(
	current projectedRow,
	y, width int,
) string {
	candidate := diffCursor{row: y, pane: v.cursor.pane}
	line, _ := v.line(candidate)
	prefix := lineMarker(line.Kind)
	gutter := fmt.Sprintf(
		"%*s %*s %s ",
		lineNumberWidth,
		number(line.OldNumber),
		lineNumberWidth,
		number(line.NewNumber),
		prefix,
	)
	return v.renderCodeLine(
		gutter,
		current.panes[right].text,
		width,
		line,
		candidate,
	)
}

func renderFileRow(path string, width int) string {
	return fileStyle.Width(frame.ScreenWidth(width)).Render(path)
}

func (v diffRenderer) renderSplitRow(
	current projectedRow,
	y int,
	leftWidth, rightWidth int,
) string {
	left := v.renderPane(
		current,
		y,
		left,
		leftWidth,
	)
	right := v.renderPane(
		current,
		y,
		right,
		rightWidth,
	)
	return left + paneSeparator + right
}

func (v diffRenderer) renderPane(
	current projectedRow,
	y int,
	pane diffPane,
	width int,
) string {
	candidate := diffCursor{row: y, pane: pane}
	line, ok := v.line(candidate)
	if !ok {
		return strings.Repeat(" ", width)
	}
	text := current.panes[pane].text
	numberValue := line.NewNumber
	if pane == left {
		numberValue = line.OldNumber
	}
	prefix := lineMarker(line.Kind) + " "
	gutter := fmt.Sprintf("%*s ", lineNumberWidth, number(numberValue))
	return v.renderCodeLine(
		gutter,
		prefix+text,
		width,
		line,
		candidate,
	)
}

func (v diffRenderer) renderCodeLine(
	gutter, text string,
	width int,
	line patch.Line,
	candidate diffCursor,
) string {
	value := gutter + fitANSIWindow(
		text, v.viewport.leftColumn, width-lipgloss.Width(gutter),
	)
	style, colors := renderedLineStyle(
		line.Kind,
		candidate,
		v.cursor,
		v.selection,
		v.styles,
	)
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

func renderedLineStyle(
	kind patch.LineKind,
	candidate diffCursor,
	cursor diffCursor,
	selection *diffSelection,
	styles lineStyles,
) (lipgloss.Style, foregroundPolicy) {
	style, colors := lineStyle(kind, styles), preserveForeground
	if selection.contains(candidate) {
		style, colors = styles.selection, discardForeground
	}
	if cursor == candidate {
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
