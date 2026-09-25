package ui

import (
	"regexp"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

type rowStyle = lipgloss.Style

func (v *diffView) Render(viewport Viewport, cursor Cursor, selection *Selection) string {
	viewport = v.clampViewport(viewport)
	rows, top, height := v.rows, viewport.Top, viewport.Height
	start := top.Y
	lines := make([]string, 0, height)
	if v.hasStickyHeader(top, height) {
		current := rows[start]
		lines = append(lines, v.renderFileRow(v.patch.Files[current.file].DisplayPath, viewport.Width))
	}
	end := min(len(rows), start+v.contentHeight(viewport))
	for y := start; y < end; y++ {
		current := rows[y]
		if v.split && current.kind == lineRow {
			lines = append(lines, v.renderSplitRow(current, y, viewport, cursor, selection))
		} else {
			lines = append(lines, v.renderUnifiedRow(current, y, viewport, cursor, selection))
		}
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	return joinLines(lines, "\n")
}

func (v *diffView) renderUnifiedRow(current entry, y int, viewport Viewport, cursor Cursor, selection *Selection) string {
	width := max(20, viewport.Width)
	text := current.text
	switch current.kind {
	case fileRow:
		return v.renderFileRow(text, width)
	case metadataRow:
		return metadataStyle.Render("  " + text)
	case hunkRow:
		return hunkStyle.Render(text)
	case lineRow:
		line := v.lineAt(current, current.rightLine)
		prefix := linePrefix(line.Kind, "")
		gutter := formatString("%5s %5s %s ", number(line.OldNumber), number(line.NewNumber), prefix)
		value := gutter + fitANSIWindow(text, viewport.LeftColumn, width-widthOf(gutter))
		candidate := lineCursor(y, cursor.Pane)
		return v.renderLineRow(line, value, width, candidate, cursor.Coordinate.Y == y, selection)
	}
	return ""
}

func (v *diffView) renderFileRow(path string, width int) string {
	return fileStyle.Width(max(20, width)).Render(path)
}

func (v *diffView) renderSplitRow(current entry, y int, viewport Viewport, cursor Cursor, selection *Selection) string {
	width, offset := viewport.Width, viewport.LeftColumn
	leftWidth := max(20, (width-3)/2)
	rightWidth := max(20, width-3-leftWidth)
	renderPane := v.renderPane
	left := renderPane(current, y, Left, leftWidth, offset, cursor, selection)
	right := renderPane(current, y, Right, rightWidth, offset, cursor, selection)
	return left + " │ " + right
}

func (v *diffView) renderPane(current entry, y int, pane Pane, width, offset int, cursor Cursor, selection *Selection) string {
	index := v.lineIndex(current, pane)
	if index < 0 {
		return repeat(" ", width)
	}
	line := v.lineAt(current, index)
	text := current.right
	numberValue := line.NewNumber
	if pane == Left {
		text, numberValue = current.left, line.OldNumber
	}
	prefix := linePrefix(line.Kind, " ")
	gutter := formatString("%5s ", number(numberValue))
	value := gutter + fitANSIWindow(prefix+text, offset, width-widthOf(gutter))
	candidate := lineCursor(y, pane)
	return v.renderLineRow(line, value, width, candidate, cursor == candidate, selection)
}

func (v *diffView) lineAt(current entry, index int) diffLine {
	return v.patch.Files[current.file].Hunks[current.hunk].Lines[index]
}

func (v *diffView) renderLineRow(line diffLine, value string, width int, candidate Cursor, active bool, selection *Selection) string {
	dark := v.dark
	style := lineStyle(line.Kind, dark)
	strip := false
	if selected(selection, candidate) {
		style, strip = selectionRowStyle(dark), true
	}
	if active {
		style, strip = cursorStyle, true
	}
	return renderStyledRow(style, value, width, strip)
}

func linePrefix(kind diffLineKind, gap string) string {
	switch kind {
	case patch.Addition:
		return addedStyle.Render("+") + gap
	case patch.Deletion:
		return removedStyle.Render("-") + gap
	default:
		return " " + gap
	}
}

func selected(selection *Selection, cursor Cursor) bool {
	if selection == nil {
		return false
	}
	first, last := selection.First, selection.Last
	y, pane := cursor.Coordinate.Y, cursor.Pane
	firstPane, lastPane := first.Pane, last.Pane
	start, end := first.Coordinate.Y, last.Coordinate.Y
	if start == end && firstPane != lastPane {
		return y == start && (pane == firstPane || pane == lastPane)
	}
	if firstPane != pane {
		return false
	}
	if start > end {
		start, end = end, start
	}
	return y >= start && y <= end
}

func lineStyle(kind diffLineKind, dark bool) rowStyle {
	lightDark := lipgloss.LightDark(dark)
	color := lipgloss.Color
	background := lipgloss.NewStyle().Background
	switch kind {
	case patch.Addition:
		return background(lightDark(color("#dafbe1"), color("#1b3823")))
	case patch.Deletion:
		return background(lightDark(color("#ffebe9"), color("#402222")))
	default:
		return contextStyle
	}
}

func selectionRowStyle(dark bool) rowStyle {
	color := lipgloss.Color
	return lipgloss.NewStyle().Background(lipgloss.LightDark(dark)(color("#dbeafe"), color("#1e3a5f")))
}

func number(value diffLineNumber) string {
	if value == 0 {
		return ""
	}
	return strconv.Itoa(int(value))
}

func renderStyledRow(style rowStyle, value string, width int, stripForeground bool) string {
	value = filterANSIColors(value, stripForeground)
	fitted := fitANSIWindow(value, 0, width)
	prefix := stylePrefix(style)
	if prefix != "" {
		fitted = replaceAll(fitted, "\x1b[0m", "\x1b[0m"+prefix)
		fitted = replaceAll(fitted, "\x1b[m", "\x1b[m"+prefix)
	}
	return style.Render(fitted)
}

func filterANSIColors(value string, stripForeground bool) string {
	colors := ansiBackgroundColors
	if stripForeground {
		colors = ansiAllColors
	}
	return ansiSGRPattern.ReplaceAllStringFunc(value, func(sequence string) string {
		parameters := sequence[2 : len(sequence)-1]
		if parameters == "" {
			return sequence
		}
		parameters = colors.ReplaceAllStringFunc(parameters, keepSGRSeparator)
		if parameters == "" {
			return ""
		}
		return "\x1b[" + parameters + "m"
	})
}

func keepSGRSeparator(match string) string {
	if match[0] == ';' && match[len(match)-1] == ';' {
		return ";"
	}
	return ""
}

func fitANSIWindow(value string, offset, width int) string {
	if width <= 0 {
		return ""
	}
	value = expandTabs(value)
	if offset > 0 {
		value = ansi.TruncateLeft(value, offset, "")
	}
	value = ansi.Truncate(value, width, "")
	if padding := width - widthOf(value); padding > 0 {
		value += repeat(" ", padding)
	}
	return value
}

func expandTabs(value string) string { return replaceAll(value, "\t", "    ") }

func stylePrefix(style rowStyle) string {
	const marker = "\x00"
	rendered := style.Render(marker)
	index := strings.Index(rendered, marker)
	if index < 0 {
		return ""
	}
	return rendered[:index]
}

var (
	ansiSGRPattern       = regexp.MustCompile(`\x1b\[[0-9;]*m`)
	ansiBackgroundColors = regexp.MustCompile(`(^|;)(0*48(;2(;[^;]*){0,3}|;5(;[^;]*)?)?|0*4[0-9]|0*10[0-7])(;|$)`)
	ansiAllColors        = regexp.MustCompile(`(^|;)(0*48(;2(;[^;]*){0,3}|;5(;[^;]*)?)?|0*4[0-9]|0*10[0-7]|0*38(;2(;[^;]*){0,3}|;5(;[^;]*)?)?|0*3[0-9]|0*9[0-7])(;|$)`)
	fileStyle            = lipgloss.NewStyle().Bold(true)
	metadataStyle        = lipgloss.NewStyle().Faint(true)
	hunkStyle            = lipgloss.NewStyle().Foreground(lipgloss.Magenta)
	contextStyle         = lipgloss.NewStyle()
	addedStyle           = lipgloss.NewStyle().Foreground(lipgloss.Green)
	removedStyle         = lipgloss.NewStyle().Foreground(lipgloss.Red)
	cursorStyle          = lipgloss.NewStyle().Reverse(true)
)
