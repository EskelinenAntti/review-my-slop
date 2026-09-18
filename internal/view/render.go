package view

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

func (v *diffView) Render(viewport Viewport, cursor Cursor, selection *Selection) string {
	viewport = v.clampViewport(viewport)
	lines := make([]string, 0, viewport.Height)
	if v.hasStickyHeader(viewport.Top, viewport.Height) {
		current := v.rows[viewport.Top.Y]
		lines = append(lines, v.renderFileRow(v.patch.Files[current.fileIndex].DisplayPath, viewport.Width))
	}
	end := min(len(v.rows), viewport.Top.Y+v.contentHeight(viewport))
	for y := viewport.Top.Y; y < end; y++ {
		current := v.rows[y]
		if v.split && current.kind == lineRow {
			lines = append(lines, v.renderSplitRow(current, y, viewport, cursor, selection))
		} else {
			lines = append(lines, v.renderUnifiedRow(current, y, viewport, cursor, selection))
		}
	}
	for len(lines) < viewport.Height {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func (v *diffView) renderUnifiedRow(current row, y int, viewport Viewport, cursor Cursor, selection *Selection) string {
	width := max(minimumDiffWidth, viewport.Width)
	switch current.kind {
	case fileRow:
		return v.renderFileRow(current.text, width)
	case metadataRow:
		return metadataStyle.Render("  " + current.text)
	case hunkRow:
		return hunkStyle.Render(current.text)
	case lineRow:
		line := v.patch.Files[current.fileIndex].Hunks[current.hunkIndex].Lines[current.rightIndex]
		prefix := " "
		if line.Kind == patch.Addition {
			prefix = addedStyle.Render("+")
		}
		if line.Kind == patch.Deletion {
			prefix = removedStyle.Render("-")
		}
		gutter := fmt.Sprintf("%5s %5s %s ", number(line.OldNumber), number(line.NewNumber), prefix)
		value := gutter + fitANSIWindow(current.text, viewport.LeftColumn, width-lipgloss.Width(gutter))
		candidate := Cursor{Coordinate: Coordinate{Y: y}, Pane: cursor.Pane}
		style, strip := v.rowStyle(line.Kind, candidate, cursor, selection)
		return renderStyledRow(style, value, width, strip)
	}
	return ""
}

func (v *diffView) renderFileRow(path string, width int) string {
	return fileStyle.Width(max(minimumDiffWidth, width)).Render(path)
}

func (v *diffView) renderSplitRow(current row, y int, viewport Viewport, cursor Cursor, selection *Selection) string {
	leftWidth := max(minimumDiffWidth, (viewport.Width-splitDividerWidth)/2)
	rightWidth := max(minimumDiffWidth, viewport.Width-splitDividerWidth-leftWidth)
	left := v.renderPane(current, y, Left, leftWidth, viewport.LeftColumn, cursor, selection)
	right := v.renderPane(current, y, Right, rightWidth, viewport.LeftColumn, cursor, selection)
	return left + " │ " + right
}

func (v *diffView) renderPane(current row, y int, pane Pane, width, offset int, cursor Cursor, selection *Selection) string {
	index := v.lineIndex(current, pane)
	if index < 0 {
		return strings.Repeat(" ", width)
	}
	line := v.patch.Files[current.fileIndex].Hunks[current.hunkIndex].Lines[index]
	text := current.rightText
	numberValue := line.NewNumber
	if pane == Left {
		text, numberValue = current.leftText, line.OldNumber
	}
	prefix := "  "
	if line.Kind == patch.Addition {
		prefix = addedStyle.Render("+") + " "
	}
	if line.Kind == patch.Deletion {
		prefix = removedStyle.Render("-") + " "
	}
	gutter := fmt.Sprintf("%5s ", number(numberValue))
	value := gutter + fitANSIWindow(prefix+text, offset, width-lipgloss.Width(gutter))
	candidate := Cursor{Coordinate: Coordinate{Y: y}, Pane: pane}
	style, strip := v.rowStyle(line.Kind, candidate, cursor, selection)
	return renderStyledRow(style, value, width, strip)
}

func (v *diffView) rowStyle(kind patch.LineKind, candidate, cursor Cursor, selection *Selection) (lipgloss.Style, bool) {
	style := lineStyle(kind, v.darkBackground)
	isSelected := isSelected(selection, candidate)
	if isSelected {
		style = selectionRowStyle(v.darkBackground)
	}
	if cursor == candidate {
		return cursorStyle, true
	}
	return style, isSelected
}

func lineStyle(kind patch.LineKind, darkBackground bool) lipgloss.Style {
	lightDark := lipgloss.LightDark(darkBackground)
	switch kind {
	case patch.Addition:
		return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#dafbe1"), lipgloss.Color("#1b3823")))
	case patch.Deletion:
		return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#ffebe9"), lipgloss.Color("#402222")))
	default:
		return contextStyle
	}
}

func selectionRowStyle(darkBackground bool) lipgloss.Style {
	return lipgloss.NewStyle().Background(lipgloss.LightDark(darkBackground)(lipgloss.Color("#dbeafe"), lipgloss.Color("#1e3a5f")))
}

func number(value patch.LineNumber) string {
	if value == 0 {
		return ""
	}
	return strconv.Itoa(int(value))
}

func renderStyledRow(style lipgloss.Style, value string, width int, removeForeground bool) string {
	value = stripANSIColors(value, removeForeground)
	fitted := fitANSIWindow(value, 0, width)
	// Syntax highlighting emits resets that would otherwise remove the row's
	// selection or cursor style in the middle of a line.
	prefix := openingStyleSequence(style)
	if prefix != "" {
		fitted = strings.ReplaceAll(fitted, "\x1b[0m", "\x1b[0m"+prefix)
		fitted = strings.ReplaceAll(fitted, "\x1b[m", "\x1b[m"+prefix)
	}
	return style.Render(fitted)
}

const (
	minimumDiffWidth     = 20
	ansiForegroundCode   = 38
	ansiBackgroundCode   = 48
	ansiTrueColor        = "2"
	ansiIndexedColor     = "5"
	standardForegroundLo = 30
	standardForegroundHi = 39
	brightForegroundLo   = 90
	brightForegroundHi   = 97
	standardBackgroundLo = 40
	standardBackgroundHi = 49
	brightBackgroundLo   = 100
	brightBackgroundHi   = 107
)

func stripANSIColors(value string, removeForeground bool) string {
	return ansiSGRPattern.ReplaceAllStringFunc(value, func(sequence string) string {
		parameters := sequence[2 : len(sequence)-1]
		if parameters == "" {
			return sequence
		}
		parts := strings.Split(parameters, ";")
		filtered := make([]string, 0, len(parts))
		for index := 0; index < len(parts); index++ {
			code, err := strconv.Atoi(parts[index])
			if err != nil {
				filtered = append(filtered, parts[index])
				continue
			}
			switch {
			case code == ansiBackgroundCode || removeForeground && code == ansiForegroundCode:
				index = skipColorArguments(parts, index)
			case isBackgroundColor(code), removeForeground && isForegroundColor(code):
			default:
				filtered = append(filtered, parts[index])
			}
		}
		if len(filtered) == 0 {
			return ""
		}
		return "\x1b[" + strings.Join(filtered, ";") + "m"
	})
}

func skipColorArguments(parts []string, index int) int {
	if index+1 >= len(parts) {
		return index
	}
	switch parts[index+1] {
	case ansiTrueColor:
		return skipColorComponents(index, len(parts), 3)
	case ansiIndexedColor:
		return skipColorComponents(index, len(parts), 1)
	default:
		return index
	}
}

func skipColorComponents(index, partCount, componentCount int) int {
	return min(index+1+componentCount, partCount-1)
}

func isBackgroundColor(code int) bool {
	return code >= standardBackgroundLo && code <= standardBackgroundHi || code >= brightBackgroundLo && code <= brightBackgroundHi
}

func isForegroundColor(code int) bool {
	return code >= standardForegroundLo && code <= standardForegroundHi || code >= brightForegroundLo && code <= brightForegroundHi
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

func expandTabs(value string) string { return strings.ReplaceAll(value, "\t", "    ") }

func openingStyleSequence(style lipgloss.Style) string {
	// Rendering a marker is the only stable way to ask lipgloss for the style's
	// opening ANSI sequence; the marker itself is removed immediately.
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
