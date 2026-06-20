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
	end := min(len(v.rows), viewport.Top.Y+viewport.Height)
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

func (v *diffView) renderUnifiedRow(current entry, y int, viewport Viewport, cursor Cursor, selection *Selection) string {
	width := max(20, viewport.Width)
	switch current.kind {
	case fileRow:
		return fileStyle.Width(width).Render(current.text)
	case metadataRow:
		return metadataStyle.Render("  " + current.text)
	case hunkRow:
		return hunkStyle.Render(current.text)
	case lineRow:
		line := v.patch.Files[current.file].Hunks[current.hunk].Lines[current.rightLine]
		prefix := " "
		if line.Kind == patch.Addition {
			prefix = addedStyle.Render("+")
		}
		if line.Kind == patch.Deletion {
			prefix = removedStyle.Render("-")
		}
		gutter := fmt.Sprintf("%5s %5s %s ", number(line.OldNumber), number(line.NewNumber), prefix)
		value := gutter + fitANSIWindow(current.text, viewport.LeftColumn, width-lipgloss.Width(gutter))
		style := lineStyle(line.Kind, v.dark)
		strip := false
		candidate := Cursor{Coordinate: Coordinate{Y: y}, Pane: cursor.Pane}
		if selected(selection, candidate) {
			style, strip = selectionRowStyle(v.dark), true
		}
		if cursor.Coordinate.Y == y {
			style, strip = cursorStyle, true
		}
		return renderStyledRow(style, value, width, strip)
	}
	return ""
}

func (v *diffView) renderSplitRow(current entry, y int, viewport Viewport, cursor Cursor, selection *Selection) string {
	leftWidth := max(20, (viewport.Width-3)/2)
	rightWidth := max(20, viewport.Width-3-leftWidth)
	left := v.renderPane(current, y, Left, leftWidth, viewport.LeftColumn, cursor, selection)
	right := v.renderPane(current, y, Right, rightWidth, viewport.LeftColumn, cursor, selection)
	return left + " │ " + right
}

func (v *diffView) renderPane(current entry, y int, pane Pane, width, offset int, cursor Cursor, selection *Selection) string {
	index := v.lineIndex(current, pane)
	if index < 0 {
		return strings.Repeat(" ", width)
	}
	line := v.patch.Files[current.file].Hunks[current.hunk].Lines[index]
	text := current.right
	numberValue := line.NewNumber
	if pane == Left {
		text, numberValue = current.left, line.OldNumber
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
	style := lineStyle(line.Kind, v.dark)
	strip := false
	candidate := Cursor{Coordinate: Coordinate{Y: y}, Pane: pane}
	if selected(selection, candidate) {
		style, strip = selectionRowStyle(v.dark), true
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
	if selection.First.Pane != cursor.Pane {
		return false
	}
	if first > last {
		first, last = last, first
	}
	return cursor.Coordinate.Y >= first && cursor.Coordinate.Y <= last
}

func lineStyle(kind patch.LineKind, dark bool) lipgloss.Style {
	lightDark := lipgloss.LightDark(dark)
	switch kind {
	case patch.Addition:
		return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#dafbe1"), lipgloss.Color("#1b3823")))
	case patch.Deletion:
		return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#ffebe9"), lipgloss.Color("#402222")))
	default:
		return contextStyle
	}
}

func selectionRowStyle(dark bool) lipgloss.Style {
	return lipgloss.NewStyle().Background(lipgloss.LightDark(dark)(lipgloss.Color("#dbeafe"), lipgloss.Color("#1e3a5f")))
}

func number(value patch.LineNumber) string {
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
