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

const (
	lineNumberWidth    = 5
	paneSeparator      = " │ "
	paneSeparatorWidth = 3

	// Unified rows show two numbers and a marker; split panes show one number.
	unifiedGutterWidth = 2*lineNumberWidth + 4
	paneGutterWidth    = lineNumberWidth + 1
	// Split panes scroll the marker and its following space with the source.
	changeMarkerWidth = 2
)

// rowRenderer holds the presentation state shared by every row in one frame.
type rowRenderer struct {
	view      *diffView
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
	lines := make([]string, 0, viewport.Height)
	if v.hasStickyHeader(viewport.top, viewport.Height) {
		current := v.rows[viewport.top]
		lines = append(
			lines,
			renderFileRow(
				v.patch.Files[current.file].DisplayPath,
				viewport.Width,
			),
		)
	}
	renderer := rowRenderer{
		view: v, viewport: viewport, cursor: cursor, selection: selection,
	}
	end := min(len(v.rows), viewport.top+v.contentHeight(viewport))
	for y := viewport.top; y < end; y++ {
		current := v.rows[y]
		lines = append(
			lines,
			renderer.renderRow(current, y),
		)
	}
	for len(lines) < viewport.Height {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func (r rowRenderer) renderRow(current displayRow, y int) string {
	switch current.kind {
	case fileRow:
		return renderFileRow(current.text, r.viewport.Width)
	case metadataRow:
		return metadataStyle.Render("  " + current.text)
	case hunkRow:
		return hunkStyle.Render(current.text)
	case lineRow:
		if r.view.split {
			return r.renderSplitRow(current, y)
		}
		return r.renderUnifiedLine(current, y)
	}
	return ""
}

func (r rowRenderer) renderUnifiedLine(current displayRow, y int) string {
	width := max(20, r.viewport.Width)
	hunk := r.view.patch.Files[current.file].Hunks[current.hunk]
	line := hunk.Lines[current.rightLine]
	gutter := fmt.Sprintf(
		"%*s %*s %s ",
		lineNumberWidth,
		number(line.OldNumber),
		lineNumberWidth,
		number(line.NewNumber),
		changeMarker(line.Kind),
	)
	value := gutter + fitANSIWindow(
		current.text,
		r.viewport.LeftColumn,
		width-lipgloss.Width(gutter),
	)
	candidate := diffCursor{row: y, pane: r.cursor.pane}
	return r.renderCodeLine(line.Kind, value, width, candidate)
}

func renderFileRow(path string, width int) string {
	return fileStyle.Width(max(20, width)).Render(path)
}

func (r rowRenderer) renderSplitRow(current displayRow, y int) string {
	leftWidth, rightWidth := splitPaneWidths(r.viewport.Width)
	left := r.renderPane(current, y, left, leftWidth)
	right := r.renderPane(current, y, right, rightWidth)
	return left + paneSeparator + right
}

func splitPaneWidths(width int) (left, right int) {
	left = max(20, (width-paneSeparatorWidth)/2)
	return left, max(20, width-paneSeparatorWidth-left)
}

func (r rowRenderer) renderPane(
	current displayRow,
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
	gutter := fmt.Sprintf("%*s ", lineNumberWidth, number(numberValue))
	value := gutter + fitANSIWindow(
		changeMarker(line.Kind)+" "+text,
		r.viewport.LeftColumn,
		width-lipgloss.Width(gutter),
	)
	candidate := diffCursor{row: y, pane: pane}
	return r.renderCodeLine(line.Kind, value, width, candidate)
}

func (r rowRenderer) renderCodeLine(
	kind patch.LineKind,
	value string,
	width int,
	position diffCursor,
) string {
	style := lineStyle(kind, r.view.dark)
	colors := preserveForeground
	if selected(r.selection, position) {
		style, colors = selectionRowStyle(r.view.dark), discardForeground
	}
	if r.cursor == position {
		style, colors = cursorStyle, discardForeground
	}
	return renderStyledRow(style, value, width, colors)
}

func changeMarker(kind patch.LineKind) string {
	switch kind {
	case patch.Addition:
		return addedStyle.Render("+")
	case patch.Deletion:
		return removedStyle.Render("-")
	default:
		return " "
	}
}

func selected(selection *diffSelection, cursor diffCursor) bool {
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

type foregroundPolicy uint8

const (
	preserveForeground foregroundPolicy = iota
	discardForeground
)

func renderStyledRow(
	style lipgloss.Style,
	value string,
	width int,
	colors foregroundPolicy,
) string {
	value = filterANSIColors(value, colors)
	fitted := fitANSIWindow(value, 0, width)
	prefix := stylePrefix(style)
	if prefix != "" {
		fitted = strings.ReplaceAll(fitted, "\x1b[0m", "\x1b[0m"+prefix)
		fitted = strings.ReplaceAll(fitted, "\x1b[m", "\x1b[m"+prefix)
	}
	return style.Render(fitted)
}

func filterANSIColors(value string, colors foregroundPolicy) string {
	return ansiSGRPattern.ReplaceAllStringFunc(
		value, func(sequence string) string {
			return filterSGR(sequence, colors)
		},
	)
}

func filterSGR(sequence string, colors foregroundPolicy) string {
	parameters := sequence[2 : len(sequence)-1]
	if parameters == "" {
		return sequence
	}
	parts := strings.Split(parameters, ";")
	filtered := filterSGRParts(parts, colors)
	if len(filtered) == 0 {
		return ""
	}
	return "\x1b[" + strings.Join(filtered, ";") + "m"
}

func filterSGRParts(parts []string, colors foregroundPolicy) []string {
	filtered := make([]string, 0, len(parts))
	for index := 0; index < len(parts); index++ {
		code, err := strconv.Atoi(parts[index])
		if err != nil {
			filtered = append(filtered, parts[index])
			continue
		}
		if extendedColorRemoved(code, colors) {
			index = colorParameterEnd(parts, index)
			continue
		}
		if simpleColorRemoved(code, colors) {
			continue
		}
		filtered = append(filtered, parts[index])
	}
	return filtered
}

func extendedColorRemoved(code int, colors foregroundPolicy) bool {
	return code == 48 || colors == discardForeground && code == 38
}

func simpleColorRemoved(code int, colors foregroundPolicy) bool {
	return backgroundColor(code) ||
		colors == discardForeground && foregroundColor(code)
}

func colorParameterEnd(parts []string, index int) int {
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

func backgroundColor(code int) bool {
	return code >= 40 && code <= 49 || code >= 100 && code <= 107
}

func foregroundColor(code int) bool {
	return code >= 30 && code <= 39 || code >= 90 && code <= 97
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
	if padding := width - lipgloss.Width(value); padding > 0 {
		value += strings.Repeat(" ", padding)
	}
	return value
}

func expandTabs(value string) string {
	return strings.ReplaceAll(value, "\t", "    ")
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
