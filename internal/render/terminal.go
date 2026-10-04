package render

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/alecthomas/chroma/v2/quick"
	"github.com/charmbracelet/x/ansi"

	"github.com/eskelinenantti/review-my-slop/internal/layout"
	"github.com/eskelinenantti/review-my-slop/internal/navigation"
	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

// Theme selects syntax colors that remain legible against the terminal's
// current background.
type Theme struct {
	Dark bool
}

// Renderer reuses syntax highlighting while the same document and theme are
// being drawn. It retains one document's highlight data and drops it when the
// document or theme changes.
type Renderer struct {
	document   *layout.Document
	theme      Theme
	highlights map[int]highlightedSources
}

// NewRenderer returns a terminal renderer with no cached document.
func NewRenderer() *Renderer { return &Renderer{} }

// Terminal paints a document for a detached navigation snapshot. It does not
// change the snapshot or document.
func Terminal(d *layout.Document, state navigation.Snapshot, theme Theme) string {
	return NewRenderer().Render(d, state, theme)
}

// Render paints a document for a detached navigation snapshot. It does not
// change the snapshot or document.
func (r *Renderer) Render(d *layout.Document, state navigation.Snapshot, theme Theme) string {
	if d == nil {
		return ""
	}
	viewport := state.Viewport
	width, height := max(1, viewport.Width), max(0, viewport.Height)
	if height == 0 {
		return ""
	}
	rows := make([]string, 0, height)
	p := d.Patch()
	highlights := r.cachedHighlights(d, p, theme)
	sticky := stickyFileRow(d, viewport.Top, height)
	if sticky >= 0 {
		rows = append(rows, renderFileRow(d.Row(sticky).Text, width))
	}
	contentHeight := height - len(rows)
	end := min(d.RowCount(), max(0, viewport.Top)+contentHeight)
	for y := max(0, viewport.Top); y < end; y++ {
		row := d.Row(y)
		if d.Format() == layout.Split && row.Kind == layout.LineRow {
			rows = append(rows, renderSplitRow(p, row, y, width, viewport.LeftColumn, state, theme, highlights))
		} else {
			rows = append(rows, renderUnifiedRow(p, row, y, width, viewport.LeftColumn, state, theme, highlights))
		}
	}
	for len(rows) < height {
		rows = append(rows, "")
	}
	return strings.Join(rows, "\n")
}

func (r *Renderer) cachedHighlights(d *layout.Document, p patch.Patch, theme Theme) map[int]highlightedSources {
	if r.document == d && r.theme == theme && r.highlights != nil {
		return r.highlights
	}
	highlights := make(map[int]highlightedSources)
	for index, file := range p.Files {
		highlights[index] = highlightedSources{
			Old: highlight(file.Path(), file.OldSource, theme.Dark),
			New: highlight(file.Path(), file.NewSource, theme.Dark),
		}
	}
	r.document, r.theme, r.highlights = d, theme, highlights
	return highlights
}

func stickyFileRow(d *layout.Document, top, height int) int {
	if height <= 1 || top < 0 || top >= d.RowCount() || d.Row(top).Kind == layout.FileRow {
		return -1
	}
	for index := top; index >= 0; index-- {
		if d.Row(index).Kind == layout.FileRow {
			return index
		}
	}
	return -1
}

func renderUnifiedRow(p patch.Patch, row layout.Row, y, viewportWidth, offset int, state navigation.Snapshot, theme Theme, highlights map[int]highlightedSources) string {
	width := max(20, viewportWidth)
	switch row.Kind {
	case layout.FileRow:
		return renderFileRow(row.Text, width)
	case layout.MetadataRow:
		return metadataStyle.Render("  " + row.Text)
	case layout.HunkRow:
		return hunkStyle.Render(row.Text)
	case layout.LineRow:
		line, ok := patchLine(p, row, row.RightLine)
		if !ok {
			return ""
		}
		text := line.Text
		sources := highlights[row.File]
		if line.Kind == patch.Deletion {
			text = highlightedLine(sources.Old, line.OldNumber, line.Text)
		} else {
			text = highlightedLine(sources.New, line.NewNumber, line.Text)
		}
		prefix := " "
		if line.Kind == patch.Addition {
			prefix = addedStyle.Render("+")
		} else if line.Kind == patch.Deletion {
			prefix = removedStyle.Render("-")
		}
		gutter := fmt.Sprintf("%5s %5s %s ", number(line.OldNumber), number(line.NewNumber), prefix)
		value := gutter + fitANSIWindow(text, offset, width-lipgloss.Width(gutter))
		style := lineStyle(line.Kind, theme.Dark)
		strip := false
		if selected(state.Selection, layout.Cell{Row: y, Pane: activePane(state)}) {
			style, strip = selectionRowStyle(theme.Dark), true
		}
		if state.Cursor != nil && state.Cursor.Row == y {
			style, strip = cursorStyle, true
		}
		return renderStyledRow(style, value, width, strip)
	default:
		return ""
	}
}

func renderFileRow(path string, width int) string {
	return fileStyle.Width(max(20, width)).Render(path)
}

func renderSplitRow(p patch.Patch, row layout.Row, y, viewportWidth, offset int, state navigation.Snapshot, theme Theme, highlights map[int]highlightedSources) string {
	leftWidth := max(20, (viewportWidth-3)/2)
	rightWidth := max(20, viewportWidth-3-leftWidth)
	left := renderPane(p, row, y, layout.Left, leftWidth, offset, state, theme, highlights)
	right := renderPane(p, row, y, layout.Right, rightWidth, offset, state, theme, highlights)
	return left + " │ " + right
}

func renderPane(p patch.Patch, row layout.Row, y int, pane layout.Pane, width, offset int, state navigation.Snapshot, theme Theme, highlights map[int]highlightedSources) string {
	lineIndex := row.RightLine
	if pane == layout.Left {
		lineIndex = row.LeftLine
	}
	line, ok := patchLine(p, row, lineIndex)
	if !ok {
		return strings.Repeat(" ", width)
	}
	sources := highlights[row.File]
	text, lineNumber := highlightedLine(sources.New, line.NewNumber, line.Text), line.NewNumber
	if pane == layout.Left {
		text, lineNumber = highlightedLine(sources.Old, line.OldNumber, line.Text), line.OldNumber
	}
	prefix := "  "
	if line.Kind == patch.Addition {
		prefix = addedStyle.Render("+") + " "
	} else if line.Kind == patch.Deletion {
		prefix = removedStyle.Render("-") + " "
	}
	gutter := fmt.Sprintf("%5s ", number(lineNumber))
	value := gutter + fitANSIWindow(prefix+text, offset, width-lipgloss.Width(gutter))
	style := lineStyle(line.Kind, theme.Dark)
	cell := layout.Cell{Row: y, Pane: pane}
	strip := false
	if selected(state.Selection, cell) {
		style, strip = selectionRowStyle(theme.Dark), true
	}
	if state.Cursor != nil && *state.Cursor == cell {
		style, strip = cursorStyle, true
	}
	return renderStyledRow(style, value, width, strip)
}

func patchLine(p patch.Patch, row layout.Row, lineIndex int) (patch.Line, bool) {
	if row.File < 0 || row.File >= len(p.Files) {
		return patch.Line{}, false
	}
	file := p.Files[row.File]
	if row.Hunk < 0 || row.Hunk >= len(file.Hunks) {
		return patch.Line{}, false
	}
	lines := file.Hunks[row.Hunk].Lines
	if lineIndex < 0 || lineIndex >= len(lines) {
		return patch.Line{}, false
	}
	return lines[lineIndex], true
}

type highlightedSources struct {
	Old []string
	New []string
}

func highlight(filename, source string, dark bool) []string {
	if source == "" {
		return nil
	}
	style := "catppuccin-latte"
	if dark {
		style = "catppuccin-mocha"
	}
	var out strings.Builder
	if err := quick.Highlight(&out, source, filename, "terminal16m", style); err != nil {
		return strings.Split(strings.TrimSuffix(source, "\n"), "\n")
	}
	return strings.Split(strings.TrimSuffix(out.String(), "\n"), "\n")
}

func highlightedLine(lines []string, number patch.LineNumber, fallback string) string {
	if number <= 0 || int(number) > len(lines) {
		return fallback
	}
	return lines[int(number)-1]
}

func selected(selection *layout.Selection, cell layout.Cell) bool {
	if selection == nil {
		return false
	}
	first, last := selection.First.Row, selection.Last.Row
	if first == last && selection.First.Pane != selection.Last.Pane {
		return cell.Row == first && (cell.Pane == selection.First.Pane || cell.Pane == selection.Last.Pane)
	}
	if selection.First.Pane != cell.Pane {
		return false
	}
	if first > last {
		first, last = last, first
	}
	return cell.Row >= first && cell.Row <= last
}

func activePane(state navigation.Snapshot) layout.Pane {
	if state.Cursor != nil {
		return state.Cursor.Pane
	}
	return layout.Right
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
