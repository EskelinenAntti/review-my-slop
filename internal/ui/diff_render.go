package ui

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
	rows, top := v.rows, viewport.Top
	lines := []string{}
	if v.hasStickyHeader(viewport.Top, viewport.Height) {
		current := rows[top]
		lines = append(lines, fileStyle.Width(max(20, viewport.Width)).Render(v.patch.Files[current.file].DisplayPath))
	}
	for y := top; y < min(len(rows), top+v.contentHeight(viewport)); y++ {
		current := rows[y]
		if v.split && current.kind == lineRow {
			leftWidth := max(20, (viewport.Width-3)/2)
			rightWidth := max(20, viewport.Width-3-leftWidth)
			left := v.renderPane(current, y, Left, leftWidth, viewport.LeftColumn, cursor, selection)
			right := v.renderPane(current, y, Right, rightWidth, viewport.LeftColumn, cursor, selection)
			lines = append(lines, left+" │ "+right)
		} else {
			lines = append(lines, v.renderUnifiedRow(current, y, viewport, cursor, selection))
		}
	}
	// Integer range skips padding automatically when the viewport is full.
	for range viewport.Height - len(lines) {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func (v *diffView) renderUnifiedRow(current entry, y int, viewport Viewport, cursor Cursor, selection *Selection) string {
	width, text := max(20, viewport.Width), current.text
	switch current.kind {
	case fileRow:
		return fileStyle.Width(width).Render(text)
	case metadataRow:
		return metadataStyle.Render("  " + text)
	case hunkRow:
		return hunkStyle.Render(text)
	case lineRow:
		line := v.patch.Files[current.file].Hunks[current.hunk].Lines[current.rightLine]
		prefix := linePrefix(line.Kind)
		gutter := fmt.Sprintf("%5s %5s %s ", number(line.OldNumber), number(line.NewNumber), prefix)
		style, strip := lineStyle(line.Kind, v.dark), false
		if cursor.Coordinate == y {
			style, strip = cursorStyle, true
		} else if selected(selection, Cursor{y, cursor.Pane}) {
			style, strip = selectionRowStyle(v.dark), true
		}
		return renderStyledRow(
			style,
			gutter+fitANSIWindow(text, viewport.LeftColumn, width-lipgloss.Width(gutter)),
			width,
			strip,
		)
	}
	return ""
}

func (v *diffView) renderPane(current entry, y int, pane Pane, width, offset int, cursor Cursor, selection *Selection) string {
	index := v.lineIndex(current, pane)
	if index < 0 {
		return strings.Repeat(" ", width)
	}
	line := v.patch.Files[current.file].Hunks[current.hunk].Lines[index]
	text, numberValue := current.right, line.NewNumber
	if pane == Left {
		text, numberValue = current.left, line.OldNumber
	}
	prefix := linePrefix(line.Kind) + " "
	gutter := fmt.Sprintf("%5s ", number(numberValue))
	style, strip := lineStyle(line.Kind, v.dark), false
	if cursor == (Cursor{y, pane}) {
		style, strip = cursorStyle, true
	} else if selected(selection, Cursor{y, pane}) {
		style, strip = selectionRowStyle(v.dark), true
	}
	return renderStyledRow(
		style,
		gutter+fitANSIWindow(prefix+text, offset, width-lipgloss.Width(gutter)),
		width,
		strip,
	)
}

func selected(selection *Selection, cursor Cursor) bool {
	if selection == nil {
		return false
	}
	firstCursor, lastCursor := selection.First, selection.Last
	first, last := firstCursor.Coordinate, lastCursor.Coordinate
	if first == last && firstCursor.Pane != lastCursor.Pane {
		return cursor.Coordinate == first && (cursor.Pane == firstCursor.Pane || cursor.Pane == lastCursor.Pane)
	}
	return firstCursor.Pane == cursor.Pane &&
		cursor.Coordinate >= min(first, last) && cursor.Coordinate <= max(first, last)
}

func lineStyle(kind reviewKind, dark bool) lipgloss.Style {
	lightDark := lipgloss.LightDark(dark)
	switch kind {
	case patch.Addition:
		return baseStyle.Background(lightDark(lipgloss.Color("#dafbe1"), lipgloss.Color("#1b3823")))
	case patch.Deletion:
		return baseStyle.Background(lightDark(lipgloss.Color("#ffebe9"), lipgloss.Color("#402222")))
	}
	return contextStyle
}

func number(value reviewNumber) string {
	if value == 0 {
		return ""
	}
	return strconv.Itoa(int(value))
}

func linePrefix(kind reviewKind) string {
	switch kind {
	case patch.Addition:
		return addedStyle.Render("+")
	case patch.Deletion:
		return removedStyle.Render("-")
	}
	return " "
}

func renderStyledRow(style lipgloss.Style, value string, width int, stripForeground bool) string {
	value = filterANSIColors(value, stripForeground)
	fitted := fitANSIWindow(value, 0, width)
	const marker = "\x00"
	prefix, _, found := strings.Cut(style.Render(marker), marker)
	if !found {
		prefix = ""
	}
	fitted = strings.ReplaceAll(fitted, "\x1b[0m", "\x1b[0m"+prefix)
	fitted = strings.ReplaceAll(fitted, "\x1b[m", "\x1b[m"+prefix)
	return style.Render(fitted)
}

func filterANSIColors(value string, stripForeground bool) string {
	return ansiSGRPattern.ReplaceAllStringFunc(value, func(sequence string) string {
		parameters := sequence[2 : len(sequence)-1]
		if parameters == "" {
			return sequence
		}
		parts, filtered := strings.Split(parameters, ";"), []string{}
		for index, part := range parts {
			code, err := strconv.Atoi(part)
			if err != nil {
				filtered = append(filtered, part)
				continue
			}
			switch {
			case code == 48 || stripForeground && code == 38:
				if index+1 < len(parts) {
					mode := parts[index+1]
					if mode == "2" {
						index += 4
					} else if mode == "5" {
						index += 2
					}
				}
			case code >= 40 && code <= 49, code >= 100 && code <= 107,
				stripForeground && code >= 30 && code <= 39, stripForeground && code >= 90 && code <= 97:
			default:
				filtered = append(filtered, part)
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
	value = strings.ReplaceAll(value, "\t", "    ")
	if offset > 0 {
		value = ansi.TruncateLeft(value, offset, "")
	}
	value = ansi.Truncate(value, width, "")
	value += strings.Repeat(" ", max(0, width-lipgloss.Width(value)))
	return value
}

var (
	ansiSGRPattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)
	fileStyle      = baseStyle.Bold(true)
	metadataStyle  = baseStyle.Faint(true)
	hunkStyle      = baseStyle.Foreground(lipgloss.Magenta)
	contextStyle   = baseStyle
	addedStyle     = baseStyle.Foreground(lipgloss.Green)
	removedStyle   = baseStyle.Foreground(lipgloss.Red)
	cursorStyle    = baseStyle.Reverse(true)
)

func selectionRowStyle(dark bool) lipgloss.Style {
	return baseStyle.Background(lipgloss.LightDark(dark)(lipgloss.Color("#dbeafe"), lipgloss.Color("#1e3a5f")))
}
