package ui

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/eskelinenantti/review-my-slop/internal/diff"
)

func (p *presentation) Render(viewport Viewport, cursor Cursor, selection *Selection) string {
	viewport = p.clampViewport(viewport)
	lines := make([]string, 0, viewport.Height)
	if p.hasStickyHeader(viewport.Top, viewport.Height) {
		current := p.rows[viewport.Top.Y]
		lines = append(lines, p.renderFileRow(p.changes.Files[current.fileIndex].Path(), viewport.Width))
	}
	end := min(len(p.rows), viewport.Top.Y+p.contentHeight(viewport))
	for y := viewport.Top.Y; y < end; y++ {
		current := p.rows[y]
		if p.split && current.kind == lineRow {
			lines = append(lines, p.renderSplitRow(current, y, viewport, cursor, selection))
		} else {
			lines = append(lines, p.renderUnifiedRow(current, y, viewport, cursor, selection))
		}
	}
	for len(lines) < viewport.Height {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func (p *presentation) renderUnifiedRow(current row, y int, viewport Viewport, cursor Cursor, selection *Selection) string {
	width := max(20, viewport.Width)
	switch current.kind {
	case fileRow:
		return p.renderFileRow(current.text, width)
	case metadataRow:
		return metadataStyle.Render("  " + current.text)
	case hunkRow:
		return hunkStyle.Render(current.text)
	case lineRow:
		line := p.changes.Files[current.fileIndex].Hunks[current.hunkIndex].Lines[current.rightLine]
		prefix := " "
		if line.Kind == diff.Addition {
			prefix = addedStyle.Render("+")
		}
		if line.Kind == diff.Deletion {
			prefix = removedStyle.Render("-")
		}
		gutter := fmt.Sprintf("%5s %5s %s ", number(line.OldNumber), number(line.NewNumber), prefix)
		value := gutter + fitANSIWindow(current.text, viewport.LeftColumn, width-lipgloss.Width(gutter))
		style := lineStyle(line.Kind, p.dark)
		strip := false
		candidate := Cursor{Coordinate: Coordinate{Y: y}, Pane: cursor.Pane}
		if selected(selection, candidate) {
			style, strip = selectionRowStyle(p.dark), true
		}
		if cursor.Coordinate.Y == y {
			style, strip = cursorStyle, true
		}
		return renderStyledRow(style, value, width, strip)
	}
	return ""
}

func (p *presentation) renderFileRow(path string, width int) string {
	return fileStyle.Width(max(20, width)).Render(visibleText(path))
}

func (p *presentation) renderSplitRow(current row, y int, viewport Viewport, cursor Cursor, selection *Selection) string {
	leftWidth := max(20, (viewport.Width-3)/2)
	rightWidth := max(20, viewport.Width-3-leftWidth)
	left := p.renderPane(current, y, Left, leftWidth, viewport.LeftColumn, cursor, selection)
	right := p.renderPane(current, y, Right, rightWidth, viewport.LeftColumn, cursor, selection)
	return left + " │ " + right
}

func (p *presentation) renderPane(current row, y int, pane Pane, width, offset int, cursor Cursor, selection *Selection) string {
	index := p.lineIndex(current, pane)
	if index < 0 {
		return strings.Repeat(" ", width)
	}
	line := p.changes.Files[current.fileIndex].Hunks[current.hunkIndex].Lines[index]
	text := current.right
	numberValue := line.NewNumber
	if pane == Left {
		text, numberValue = current.left, line.OldNumber
	}
	prefix := "  "
	if line.Kind == diff.Addition {
		prefix = addedStyle.Render("+") + " "
	}
	if line.Kind == diff.Deletion {
		prefix = removedStyle.Render("-") + " "
	}
	gutter := fmt.Sprintf("%5s ", number(numberValue))
	value := gutter + fitANSIWindow(prefix+text, offset, width-lipgloss.Width(gutter))
	style := lineStyle(line.Kind, p.dark)
	strip := false
	candidate := Cursor{Coordinate: Coordinate{Y: y}, Pane: pane}
	if selected(selection, candidate) {
		style, strip = selectionRowStyle(p.dark), true
	}
	if cursor == candidate {
		style, strip = cursorStyle, true
	}
	return renderStyledRow(style, value, width, strip)
}

func selected(selection *Selection, cursor Cursor) bool {
	if selection == nil {
		return false
	}
	first, last := selection.First.Coordinate.Y, selection.Last.Coordinate.Y
	if first == last && selection.First.Pane != selection.Last.Pane {
		return cursor.Coordinate.Y == first && (cursor.Pane == selection.First.Pane || cursor.Pane == selection.Last.Pane)
	}
	low, high := selection.First, selection.Last
	if low.Coordinate.Y > high.Coordinate.Y {
		low, high = high, low
	}
	if low.Pane != cursor.Pane {
		return false
	}
	first, last = low.Coordinate.Y, high.Coordinate.Y
	return cursor.Coordinate.Y >= first && cursor.Coordinate.Y <= last
}

func lineStyle(kind diff.LineKind, dark bool) lipgloss.Style {
	lightDark := lipgloss.LightDark(dark)
	switch kind {
	case diff.Addition:
		return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#dafbe1"), lipgloss.Color("#1b3823")))
	case diff.Deletion:
		return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#ffebe9"), lipgloss.Color("#402222")))
	default:
		return contextStyle
	}
}

func selectionRowStyle(dark bool) lipgloss.Style {
	return lipgloss.NewStyle().Background(lipgloss.LightDark(dark)(lipgloss.Color("#dbeafe"), lipgloss.Color("#1e3a5f")))
}

func number(value diff.LineNumber) string {
	if value == 0 {
		return ""
	}
	return strconv.Itoa(int(value))
}

func renderStyledRow(style lipgloss.Style, value string, width int, stripForeground bool) string {
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
			case code == 48 || stripForeground && code == 38:
				if index+1 < len(parts) {
					mode := parts[index+1]
					if mode == "2" {
						index = min(index+4, len(parts)-1)
					}
					if mode == "5" {
						index = min(index+2, len(parts)-1)
					}
				}
			case code >= 40 && code <= 49, code >= 100 && code <= 107,
				stripForeground && code >= 30 && code <= 39, stripForeground && code >= 90 && code <= 97:
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

func stylePrefix(style lipgloss.Style) string {
	const marker = "\x00"
	rendered := style.Render(marker)
	index := strings.Index(rendered, marker)
	if index < 0 {
		return ""
	}
	return rendered[:index]
}

var ansiSGRPattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)

var (
	fileStyle     = lipgloss.NewStyle().Bold(true)
	metadataStyle = lipgloss.NewStyle().Faint(true)
	hunkStyle     = lipgloss.NewStyle().Foreground(lipgloss.Magenta)
	addedStyle    = lipgloss.NewStyle().Foreground(lipgloss.Green)
	removedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Red)
)

func visibleSource(value string) string {
	var result strings.Builder
	for _, char := range value {
		switch {
		case char == '\n' || char == '\t':
			result.WriteRune(char)
		case char == '\r':
			result.WriteString(`\r`)
		case char < 0x20 || char == 0x7f:
			fmt.Fprintf(&result, `\x%02x`, char)
		default:
			result.WriteRune(char)
		}
	}
	return result.String()
}

func visibleText(value string) string {
	return strings.ReplaceAll(visibleSource(value), "\n", `\n`)
}
