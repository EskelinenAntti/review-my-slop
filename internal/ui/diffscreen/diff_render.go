package diffscreen

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

func (v *diffView) formatVisibleChanges(
	viewport diffViewport,
	cursor diffCursor,
	selection *diffSelection,
) string {
	viewport = v.keepVisibleAreaOnScreen(viewport)
	lines := make([]string, 0, viewport.Height)
	if v.fileHeaderStaysAtTop(viewport.top, viewport.Height) {
		current := v.rows[viewport.top]
		lines = append(
			lines,
			formatFileHeading(
				v.patch.Files[current.file].DisplayPath,
				viewport.Width,
			),
		)
	}
	end := min(len(v.rows), viewport.top+v.visibleRowsBelowHeader(viewport))
	for y := viewport.top; y < end; y++ {
		current := v.rows[y]
		if v.split && current.kind == lineRow {
			lines = append(
				lines,
				v.formatTwoColumnRow(current, y, viewport, cursor, selection),
			)
		} else {
			lines = append(
				lines,
				v.formatChangedLineRow(current, y, viewport, cursor, selection),
			)
		}
	}
	for len(lines) < viewport.Height {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func (v *diffView) formatChangedLineRow(
	current entry,
	y int,
	viewport diffViewport,
	cursor diffCursor,
	selection *diffSelection,
) string {
	width := max(20, viewport.Width)
	switch current.kind {
	case fileRow:
		return formatFileHeading(current.text, width)
	case metadataRow:
		return metadataStyle.Render("  " + current.text)
	case hunkRow:
		return hunkStyle.Render(current.text)
	case lineRow:
		return v.formatChangedLine(
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

func (v *diffView) formatChangedLine(
	current entry,
	y int,
	viewport diffViewport,
	cursor diffCursor,
	selection *diffSelection,
	width int,
) string {
	hunk := v.patch.Files[current.file].Hunks[current.hunk]
	line := hunk.Lines[current.rightLine]
	prefix := " "
	if line.Kind == patch.Addition {
		prefix = addedStyle.Render("+")
	}
	if line.Kind == patch.Deletion {
		prefix = removedStyle.Render("-")
	}
	gutter := fmt.Sprintf(
		"%5s %5s %s ",
		formatLineNumber(line.OldNumber),
		formatLineNumber(line.NewNumber),
		prefix,
	)
	value := gutter + trimStyledText(
		current.text,
		viewport.LeftColumn,
		width-lipgloss.Width(gutter),
	)
	style := styleForChange(line.Kind, v.dark)
	colors := preserveForeground
	candidate := diffCursor{row: y, pane: cursor.pane}
	if lineIsSelected(selection, candidate) {
		style, colors = styleForSelection(v.dark), discardForeground
	}
	if cursor.row == y {
		style, colors = cursorStyle, discardForeground
	}
	return formatStyledLine(style, value, width, colors)
}

func formatFileHeading(path string, width int) string {
	return fileStyle.Width(max(20, width)).Render(path)
}

func (v *diffView) formatTwoColumnRow(
	current entry,
	y int,
	viewport diffViewport,
	cursor diffCursor,
	selection *diffSelection,
) string {
	leftWidth, rightWidth := twoColumnWidths(viewport.Width)
	left := v.formatColumn(
		current,
		y,
		left,
		leftWidth,
		viewport.LeftColumn,
		cursor,
		selection,
	)
	right := v.formatColumn(
		current,
		y,
		right,
		rightWidth,
		viewport.LeftColumn,
		cursor,
		selection,
	)
	return left + " │ " + right
}

func twoColumnWidths(width int) (left, right int) {
	left = max(20, (width-3)/2)
	return left, max(20, width-3-left)
}

func (v *diffView) formatColumn(
	current entry,
	y int,
	pane diffPane,
	width, offset int,
	cursor diffCursor,
	selection *diffSelection,
) string {
	index := v.findLinePositionInChangeBlock(current, pane)
	if index < 0 {
		return strings.Repeat(" ", width)
	}
	line := v.patch.Files[current.file].Hunks[current.hunk].Lines[index]
	text := current.right
	numberValue := line.NewNumber
	if pane == left {
		text, numberValue = current.left, line.OldNumber
	}
	prefix := "  "
	if line.Kind == patch.Addition {
		prefix = addedStyle.Render("+") + " "
	}
	if line.Kind == patch.Deletion {
		prefix = removedStyle.Render("-") + " "
	}
	gutter := fmt.Sprintf("%5s ", formatLineNumber(numberValue))
	value := gutter + trimStyledText(
		prefix+text,
		offset,
		width-lipgloss.Width(gutter),
	)
	style := styleForChange(line.Kind, v.dark)
	colors := preserveForeground
	candidate := diffCursor{row: y, pane: pane}
	if lineIsSelected(selection, candidate) {
		style, colors = styleForSelection(v.dark), discardForeground
	}
	if cursor == candidate {
		style, colors = cursorStyle, discardForeground
	}
	return formatStyledLine(style, value, width, colors)
}

func lineIsSelected(selection *diffSelection, cursor diffCursor) bool {
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

func styleForChange(kind patch.LineKind, dark bool) lipgloss.Style {
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

func styleForSelection(dark bool) lipgloss.Style {
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

type foregroundPolicy uint8

const (
	preserveForeground foregroundPolicy = iota
	discardForeground
)

func formatStyledLine(
	style lipgloss.Style,
	value string,
	width int,
	colors foregroundPolicy,
) string {
	value = removeTerminalColors(value, colors)
	fitted := trimStyledText(value, 0, width)
	prefix := openingStyleCode(style)
	if prefix != "" {
		fitted = strings.ReplaceAll(fitted, "\x1b[0m", "\x1b[0m"+prefix)
		fitted = strings.ReplaceAll(fitted, "\x1b[m", "\x1b[m"+prefix)
	}
	return style.Render(fitted)
}

func removeTerminalColors(value string, colors foregroundPolicy) string {
	return ansiSGRPattern.ReplaceAllStringFunc(
		value, func(sequence string) string {
			return removeStyleCodes(sequence, colors)
		},
	)
}

func removeStyleCodes(sequence string, colors foregroundPolicy) string {
	parameters := sequence[2 : len(sequence)-1]
	if parameters == "" {
		return sequence
	}
	parts := strings.Split(parameters, ";")
	filtered := removeStyleCodeParts(parts, colors)
	if len(filtered) == 0 {
		return ""
	}
	return "\x1b[" + strings.Join(filtered, ";") + "m"
}

func removeStyleCodeParts(parts []string, colors foregroundPolicy) []string {
	filtered := make([]string, 0, len(parts))
	for index := 0; index < len(parts); index++ {
		code, err := strconv.Atoi(parts[index])
		if err != nil {
			filtered = append(filtered, parts[index])
			continue
		}
		if removeExtendedColor(code, colors) {
			index = lastColorParameter(parts, index)
			continue
		}
		if removeBasicColor(code, colors) {
			continue
		}
		filtered = append(filtered, parts[index])
	}
	return filtered
}

func removeExtendedColor(code int, colors foregroundPolicy) bool {
	return code == 48 || colors == discardForeground && code == 38
}

func removeBasicColor(code int, colors foregroundPolicy) bool {
	return isBackgroundColor(code) ||
		colors == discardForeground && isForegroundColor(code)
}

func lastColorParameter(parts []string, index int) int {
	if index+1 >= len(parts) {
		return index
	}
	switch parts[index+1] {
	case "2":
		return min(index+4, len(parts)-1)
	case "5":
		return min(index+2, len(parts)-1)
	default:
		return index
	}
}

func isBackgroundColor(code int) bool {
	return code >= 40 && code <= 49 || code >= 100 && code <= 107
}

func isForegroundColor(code int) bool {
	return code >= 30 && code <= 39 || code >= 90 && code <= 97
}

func trimStyledText(value string, offset, width int) string {
	if width <= 0 {
		return ""
	}
	value = replaceTabsWithSpaces(value)
	if offset > 0 {
		value = ansi.TruncateLeft(value, offset, "")
	}
	value = ansi.Truncate(value, width, "")
	if padding := width - lipgloss.Width(value); padding > 0 {
		value += strings.Repeat(" ", padding)
	}
	return value
}

func replaceTabsWithSpaces(value string) string {
	return strings.ReplaceAll(value, "\t", "    ")
}

func openingStyleCode(style lipgloss.Style) string {
	const marker = "\x00"
	rendered := style.Render(marker)
	index := strings.Index(rendered, marker)
	if index < 0 {
		return ""
	}
	return rendered[:index]
}

var (
	ansiSGRPattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)
	fileStyle      = lipgloss.NewStyle().Bold(true)
	metadataStyle  = lipgloss.NewStyle().Faint(true)
	hunkStyle      = lipgloss.NewStyle().Foreground(lipgloss.Magenta)
	contextStyle   = lipgloss.NewStyle()
	addedStyle     = lipgloss.NewStyle().Foreground(lipgloss.Green)
	removedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Red)
	cursorStyle    = lipgloss.NewStyle().Reverse(true)
)
