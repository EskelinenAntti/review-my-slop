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

func (v *diffView) render(
	initialViewport diffViewport,
	cursor diffCursor,
	selection *diffSelection,
) string {
	viewport := initialViewport

	viewport = v.clampViewport(viewport)
	lines := make([]string, 0, viewport.Height)
	if v.hasStickyHeader(viewport.top, viewport.Height) {
		current := v.rows[viewport.top]
		lines = append(
			lines,
			v.renderFileRow(
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
				v.renderSplitRow(current, y, viewport, cursor, selection),
			)
		} else {
			lines = append(
				lines,
				v.renderUnifiedRow(current, y, viewport, cursor, selection),
			)
		}
	}
	for len(lines) < viewport.Height {
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
	width := max(minimumWidth, viewport.Width)
	switch current.kind {
	case fileRow:
		return v.renderFileRow(current.text, width)
	case metadataRow:
		return metadataStyle.Render("  " + current.text)
	case hunkRow:
		return hunkStyle.Render(current.text)
	case lineRow:
		return v.renderUnifiedCode(current, y, viewport, cursor, selection)
	}
	return ""
}

func (*diffView) renderFileRow(path string, width int) string {
	return fileStyle.Width(max(minimumWidth, width)).Render(path)
}

func (v *diffView) renderSplitRow(
	current entry,
	y int,
	viewport diffViewport,
	cursor diffCursor,
	selection *diffSelection,
) string {
	leftWidth, rightWidth := splitPaneWidths(viewport.Width)
	left := v.renderPane(
		current,
		y,
		left,
		leftWidth,
		viewport.LeftColumn,
		cursor,
		selection,
	)
	right := v.renderPane(
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

func splitPaneWidths(width int) (leftWidth, rightWidth int) {
	left := max(minimumWidth, (width-paneDividerWidth)/int(paneCount))
	return left, max(minimumWidth, width-paneDividerWidth-left)
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
		return strings.Repeat(space, width)
	}
	line := v.patch.Files[current.file].Hunks[current.hunk].Lines[index]
	text := current.right
	numberValue := line.NewNumber
	if pane == left {
		text, numberValue = current.left, line.OldNumber
	}
	prefix := "  "
	if line.Kind == patch.Addition {
		prefix = addedStyle.Render("+") + space
	}
	if line.Kind == patch.Deletion {
		prefix = removedStyle.Render("-") + space
	}
	gutter := fmt.Sprintf("%5s ", number(numberValue))
	value := gutter +
		fitANSIWindow(prefix+text, offset, width-lipgloss.Width(gutter))
	style := lineStyle(line.Kind, v.dark)
	strip := false
	candidate := diffCursor{row: y, pane: pane}
	if selected(selection, candidate) {
		style, strip = selectionRowStyle(v.dark), true
	}
	if cursor == candidate {
		style, strip = cursorStyle, true
	}
	return renderStyledRow(style, value, width, strip)
}

func selected(selection *diffSelection, cursor diffCursor) bool {
	if selection == nil {
		return false
	}
	first, last := selection.First.row, selection.Last.row
	if first == last && selection.First.pane != selection.Last.pane {
		return selectedAcrossPanes(*selection, cursor)
	}
	if selection.First.pane != cursor.pane {
		return false
	}
	if first > last {
		first, last = last, first
	}
	return cursor.row >= first && cursor.row <= last
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

func number(value patch.LineNumber) string {
	if value == 0 {
		return ""
	}
	return strconv.Itoa(int(value))
}

func renderStyledRow(
	style lipgloss.Style,
	initialValue string,
	width int,
	stripForeground bool,
) string {
	value := initialValue

	value = filterANSIColors(value, stripForeground)
	fitted := fitANSIWindow(value, 0, width)
	prefix := stylePrefix(style)
	if prefix != "" {
		fitted = strings.ReplaceAll(fitted, "\x1b[0m", "\x1b[0m"+prefix)
		fitted = strings.ReplaceAll(fitted, "\x1b[m", "\x1b[m"+prefix)
	}
	return style.Render(fitted)
}

func filterANSIColors(value string, stripForeground bool) string {
	filter := colorFilter{foreground: stripForeground}
	return ansiSGRPattern.ReplaceAllStringFunc(value, filter.sequence)
}

type colorFilter struct{ foreground bool }

func (f colorFilter) sequence(sequence string) string {
	parameters := sequence[sgrPrefixLength : len(sequence)-1]
	if parameters == "" {
		return sequence
	}
	parts := strings.Split(parameters, ";")
	filtered := f.filterParts(parts)
	if len(filtered) == 0 {
		return ""
	}
	return "\x1b[" + strings.Join(filtered, ";") + "m"
}

func (f colorFilter) extended(code int) bool {
	return code == sgrExtendedBackground ||
		f.foreground && code == sgrExtendedForeground
}

func (f colorFilter) basic(code int) bool {
	return backgroundCode(code) || f.foreground && foregroundCode(code)
}

func backgroundCode(code int) bool {
	return code >= sgrBackgroundStart && code <= sgrDefaultBackground ||
		code >= sgrBrightBackgroundStart && code <= sgrBrightBackgroundEnd
}

func foregroundCode(code int) bool {
	return code >= sgrForegroundStart && code <= sgrDefaultForeground ||
		code >= sgrBrightForegroundStart && code <= sgrBrightForegroundEnd
}

func extendedColorEnd(parts []string, index int) int {
	if index+1 >= len(parts) {
		return index
	}
	switch parts[index+1] {
	case "2":
		return min(index+sgrRGBParameterCount, len(parts)-1)
	case "5":
		return min(index+sgrIndexedParameterCount, len(parts)-1)
	default:
		return index
	}
}

func fitANSIWindow(initialValue string, offset, width int) string {
	value := initialValue

	if width <= 0 {
		return ""
	}
	value = expandTabs(value)
	if offset > 0 {
		value = ansi.TruncateLeft(value, offset, "")
	}
	value = ansi.Truncate(value, width, "")
	if padding := width - lipgloss.Width(value); padding > 0 {
		value += strings.Repeat(space, padding)
	}
	return value
}

func expandTabs(value string) string {
	return strings.ReplaceAll(value, "\t", " ")
}

func stylePrefix(style lipgloss.Style) string {
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

func selectedAcrossPanes(selection diffSelection, cursor diffCursor) bool {
	return cursor.row == selection.First.row &&
		(cursor.pane == selection.First.pane ||
			cursor.pane == selection.Last.pane)
}

func (v *diffView) renderUnifiedCode(
	current entry,
	y int,
	viewport diffViewport,
	cursor diffCursor,
	selection *diffSelection,
) string {
	width := max(minimumWidth, viewport.Width)

	hunk := v.patch.Files[current.file].Hunks[current.hunk]
	line := hunk.Lines[current.rightLine]
	prefix := space
	if line.Kind == patch.Addition {
		prefix = addedStyle.Render("+")
	}
	if line.Kind == patch.Deletion {
		prefix = removedStyle.Render("-")
	}
	gutter := fmt.Sprintf(
		"%5s %5s %s ",
		number(line.OldNumber),
		number(line.NewNumber),
		prefix,
	)
	value := gutter +
		fitANSIWindow(
			current.text,
			viewport.LeftColumn,
			width-lipgloss.Width(gutter),
		)
	style := lineStyle(line.Kind, v.dark)
	strip := false
	candidate := diffCursor{row: y, pane: cursor.pane}
	if selected(selection, candidate) {
		style, strip = selectionRowStyle(v.dark), true
	}
	if cursor.row == y {
		style, strip = cursorStyle, true
	}
	return renderStyledRow(style, value, width, strip)
}

func (f colorFilter) filterParts(parts []string) []string {
	filtered := make([]string, 0, len(parts))
	for index := 0; index < len(parts); index++ {
		code, err := strconv.Atoi(parts[index])
		if err != nil {
			filtered = append(filtered, parts[index])
			continue
		}
		if f.extended(code) {
			index = extendedColorEnd(parts, index)
			continue
		}
		if !f.basic(code) {
			filtered = append(filtered, parts[index])
		}
	}
	return filtered
}
