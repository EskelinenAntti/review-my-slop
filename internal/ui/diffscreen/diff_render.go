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

func (v *diffView) render(
	viewport diffViewport,
	cursor diffCursor,
	selection *diffSelection,
) string {
	viewport = v.clampViewport(viewport)
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
	end := min(len(v.rows), viewport.top+v.contentHeight(viewport))
	for y := viewport.top; y < end; y++ {
		current := v.rows[y]
		if v.split && current.kind == lineRow {
			lines = append(
				lines,
				v.renderSplitRow(
					current,
					y,
					viewport,
					leftWidth,
					rightWidth,
					cursor,
					selection,
				),
			)
		} else {
			lines = append(
				lines,
				v.renderUnifiedRow(current, y, viewport, cursor, selection),
			)
		}
	}
	for len(lines) < viewport.height {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func (v *diffView) renderUnifiedRow(
	current entry,
	y int,
	viewport diffViewport,
	cursor diffCursor,
	selection *diffSelection,
) string {
	width := frame.ScreenWidth(viewport.width)
	switch current.kind {
	case fileRow:
		return renderFileRow(current.text, width)
	case metadataRow:
		return metadataStyle.Render("  " + current.text)
	case hunkRow:
		return hunkStyle.Render(current.text)
	case lineRow:
		return v.renderUnifiedLine(
			current,
			y,
			viewport,
			cursor,
			selection,
			width,
		)
	}
	return ""
}

func (v *diffView) renderUnifiedLine(
	current entry,
	y int,
	viewport diffViewport,
	cursor diffCursor,
	selection *diffSelection,
	width int,
) string {
	hunk := v.patch.Files[current.file].Hunks[current.hunk]
	line := hunk.Lines[current.rightLine]
	prefix := lineMarker(line.Kind)
	gutter := fmt.Sprintf(
		"%5s %5s %s ",
		number(line.OldNumber),
		number(line.NewNumber),
		prefix,
	)
	value := gutter + fitANSIWindow(
		current.text,
		viewport.leftColumn,
		width-lipgloss.Width(gutter),
	)
	candidate := diffCursor{row: y, pane: cursor.pane}
	style, colors := renderedLineStyle(
		line.Kind,
		candidate,
		cursor,
		selection,
		v.styles,
	)
	return renderStyledRow(style, value, width, colors)
}

func renderFileRow(path string, width int) string {
	return fileStyle.Width(frame.ScreenWidth(width)).Render(path)
}

func (v *diffView) renderSplitRow(
	current entry,
	y int,
	viewport diffViewport,
	leftWidth, rightWidth int,
	cursor diffCursor,
	selection *diffSelection,
) string {
	left := v.renderPane(
		current,
		y,
		left,
		leftWidth,
		viewport.leftColumn,
		cursor,
		selection,
	)
	right := v.renderPane(
		current,
		y,
		right,
		rightWidth,
		viewport.leftColumn,
		cursor,
		selection,
	)
	return left + " │ " + right
}

func splitPaneWidths(width int) (left, right int) {
	left = max(20, (width-3)/2)
	return left, max(20, width-3-left)
}

func (v *diffView) renderPane(
	current entry,
	y int,
	pane diffPane,
	width, offset int,
	cursor diffCursor,
	selection *diffSelection,
) string {
	index := v.lineIndex(current, pane)
	if index < 0 {
		return strings.Repeat(" ", width)
	}
	line := v.patch.Files[current.file].Hunks[current.hunk].Lines[index]
	text := current.right
	numberValue := line.NewNumber
	if pane == left {
		text, numberValue = current.left, line.OldNumber
	}
	prefix := lineMarker(line.Kind) + " "
	gutter := fmt.Sprintf("%5s ", number(numberValue))
	value := gutter + fitANSIWindow(
		prefix+text,
		offset,
		width-lipgloss.Width(gutter),
	)
	candidate := diffCursor{row: y, pane: pane}
	style, colors := renderedLineStyle(
		line.Kind,
		candidate,
		cursor,
		selection,
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
	if selectionContains(selection, candidate) {
		style, colors = styles.selection, discardForeground
	}
	if cursor == candidate {
		style, colors = cursorStyle, discardForeground
	}
	return style, colors
}

func selectionContains(selection *diffSelection, cursor diffCursor) bool {
	if selection == nil {
		return false
	}
	first, last := selection.First, selection.Last
	if first.row == last.row && first.pane != last.pane {
		return cursor.row == first.row &&
			(cursor.pane == first.pane || cursor.pane == last.pane)
	}
	if first.pane != cursor.pane {
		return false
	}
	return cursor.row >= min(
		first.row,
		last.row,
	) && cursor.row <= max(first.row, last.row)
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
