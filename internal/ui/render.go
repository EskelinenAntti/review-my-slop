package ui

import (
	"fmt"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	patches "github.com/eskelinenantti/review-my-slop/internal/patch"
)

func patchChangeCounts(patch patches.Patch) (added, removed int) {
	for _, file := range patch.Files {
		fileAdded, fileRemoved := fileChangeCounts(file)
		added += fileAdded
		removed += fileRemoved
	}
	return added, removed
}

func fileChangeCounts(file patches.File) (added, removed int) {
	for _, hunk := range file.Hunks {
		hunkAdded, hunkRemoved := hunkChangeCounts(hunk.Lines)
		added += hunkAdded
		removed += hunkRemoved
	}
	return added, removed
}

func hunkChangeCounts(lines []patches.Line) (added, removed int) {
	for _, line := range lines {
		switch line.Kind {
		case patches.Addition:
			added++
		case patches.Deletion:
			removed++
		default:
			continue
		}
	}
	return added, removed
}

func renderBody(screen *Diff) []string {
	if len(screen.view.patch.Files) != 0 {
		return strings.Split(renderPatch(
			screen.view,
			screen.viewport,
			screen.cursor,
			screen.selection,
		), "\n")
	}
	empty := "No unstaged or untracked changes."
	if screen.view.patch.Kind == patches.Branch {
		empty = "No branch or worktree changes."
	}
	body := make([]string, max(1, screen.height-3))
	body[min(1, len(body)-1)] = mutedStyle.Render(empty)
	return body
}

func status(screen *Diff) string {
	if screen.search.active {
		status := "/" + screen.search.query + cursorStyle.Render(" ")
		if screen.search.miss {
			status += errorStyle.Render("  no matches")
		}
		return status
	}
	if screen.selection != nil {
		return "visual selection  j/k extend  c comment  Esc cancel"
	}
	return "j/k/h/l move  c comment  ? help  q quit"
}

func renderFooter(screen *Diff, err error) string {
	footer := mutedStyle.Render(status(screen))
	if screen.search.repeatMiss {
		footer = errorStyle.Render(fmt.Sprintf(
			"no matches for %q",
			screen.search.term,
		))
	}
	if err != nil {
		footer = errorStyle.Render(strings.ReplaceAll(
			err.Error(),
			"\n",
			" ",
		))
	}
	return footerWithLabel(screen, footer)
}

func footerWithLabel(screen *Diff, footer string) string {
	label := footerLabel(screen.view.patch)
	if screen.viewport.top > 0 {
		label += fmt.Sprintf(" (%d%%)", viewportPercent(
			screen.view,
			screen.viewport,
		))
	}
	right := mutedStyle.Render(label)
	width := max(20, screen.width)
	footer = ansi.Truncate(footer, max(0, width-lipgloss.Width(right)-1), "")
	padding := strings.Repeat(" ", max(1, width-lipgloss.Width(
		footer,
	)-lipgloss.Width(right)))
	return footer + padding + right
}

func footerLabel(patch patches.Patch) string {
	if patch.Kind == patches.Branch {
		return "branch changes from " + patch.Branch
	}
	return "local changes"
}

func viewportPercent(view *diffView,
	viewport diffViewport) int {
	if len(view.rows) == 0 {
		return 0
	}
	bottom := min(len(
		view.rows,
	), viewport.top+viewportHeight(
		view,
		viewport,
	))
	return bottom * 100 / len(view.rows)
}

func renderFrame(header string, body []string, footer string,
	height int) string {
	bodyHeight := max(1, height-3)
	lines := make([]string, 0, bodyHeight+3)
	lines = append(lines, header)
	lines = append(lines, body[:min(len(body), bodyHeight)]...)
	for len(lines) < bodyHeight+1 {
		lines = append(lines, "")
	}
	lines = append(lines, footer, "")
	return strings.Join(lines, "\n")
}

func renderPatch(
	view *diffView,
	viewport diffViewport,
	cursor diffCursor,
	selection *diffSelection,
) string {
	viewport = viewportClamp(view, &viewport)
	lines := renderViewportHeader(view, viewport)
	lines = append(lines, renderVisibleRows(
		view, viewport, cursor, selection,
	)...)
	for len(lines) < viewport.Height {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func renderViewportHeader(view *diffView,
	viewport diffViewport) []string {
	if !viewportHasFileHeader(view, viewport) {
		return nil
	}
	entry := view.rows[viewport.top]
	path := view.patch.Files[entry.file].DisplayPath
	header := fileStyle.Width(max(20, viewport.Width)).Render(path)
	return []string{header}
}

func renderVisibleRows(
	view *diffView,
	viewport diffViewport,
	cursor diffCursor,
	selection *diffSelection,
) []string {
	end := viewportRowsEnd(view, viewport)
	lines := make([]string, 0, max(0, end-viewport.top))
	for row := viewport.top; row < end; row++ {
		entry := view.rows[row]
		lines = append(lines, renderViewportRow(
			view, entry, row, viewport, cursor, selection,
		))
	}
	return lines
}

func viewportRowsEnd(view *diffView,
	viewport diffViewport) int {
	height := viewportContentHeight(view, viewport)
	return min(len(view.rows), viewport.top+max(1, height))
}

func renderViewportRow(
	view *diffView,
	entry entry,
	row int,
	viewport diffViewport,
	cursor diffCursor,
	selection *diffSelection,
) string {
	if view.split && entry.kind == lineRow {
		return renderSplitRow(
			view,
			entry,
			row,
			viewport,
			cursor,
			selection,
		)
	}
	return renderUnifiedRow(
		entry,
		cursor,
		selection,
		view,
		viewport,
		row,
	)
}

func renderDiffCell(
	view *diffView,
	entry entry,
	row int,
	pane diffPane,
	width, offset int,
	cursor diffCursor,
	selection *diffSelection,
) string {
	lineIndex := entryLineIndex(entry, pane)
	if lineIndex < 0 {
		return strings.Repeat(" ", width)
	}
	line := view.patch.Files[entry.file].Hunks[entry.hunk].Lines[lineIndex]
	text, number := entryLineText(entry, line, pane)
	gutter := numberGutter(number)
	value := gutter + fitText(
		linePrefix(line.Kind)+text,
		width-lipgloss.Width(gutter),
		offset,
	)
	candidate := diffCursor{row: row, pane: pane}
	style := lineStyle(line.Kind, view.dark)
	colors := preserveForeground
	if selectionContainsCursor(selection, candidate) {
		style = selectionStyle(view.dark)
		colors = discardForeground
	}
	if cursor == candidate {
		style = cursorStyle
		colors = discardForeground
	}
	return renderStyledCell(value, width, 0, style, colors)
}

func entryLineIndex(entry entry,
	pane diffPane) int {
	if entry.kind != lineRow {
		return -1
	}
	if pane == right {
		return entry.rightLine
	}
	return entry.leftLine
}

func entryLineText(entry entry, line patches.Line,
	pane diffPane) (string, patches.LineNumber) {
	if pane == left {
		return entry.left, line.OldNumber
	}
	return entry.right, line.NewNumber
}

func numberGutter(number patches.LineNumber) string {
	value := ""
	if number != 0 {
		value = strconv.Itoa(int(number))
	}
	return fmt.Sprintf("%5s ", value)
}

func linePrefix(kind patches.LineKind) string {
	switch kind {
	case patches.Addition:
		return addedStyle.Render("+") + " "
	case patches.Deletion:
		return removedStyle.Render("-") + " "
	default:
		return "  "
	}
}

func renderSplitRow(
	view *diffView,
	entry entry,
	row int,
	viewport diffViewport,
	cursor diffCursor,
	selection *diffSelection,
) string {
	leftWidth := max(20, (viewport.Width-3)/2)
	rightWidth := max(20, viewport.Width-3-leftWidth)
	left := renderDiffCell(
		view, entry, row, left, leftWidth,
		viewport.LeftColumn, cursor, selection,
	)
	right := renderDiffCell(
		view, entry, row, right, rightWidth,
		viewport.LeftColumn, cursor, selection,
	)
	return left + " │ " + right
}

func renderUnifiedRow(
	current entry,
	cursor diffCursor,
	selection *diffSelection,
	view *diffView,
	viewport diffViewport,
	row int,
) string {
	width := max(20, viewport.Width)
	switch current.kind {
	case fileRow:
		return fileStyle.Width(width).Render(current.text)
	case metadataRow:
		return metadataStyle.Render("  " + current.text)
	case hunkRow:
		return hunkStyle.Render(current.text)
	case lineRow:
		return renderUnifiedLine(
			view, current, row, viewport, cursor, selection,
		)
	default:
		return ""
	}
}

func renderUnifiedLine(
	view *diffView,
	entry entry,
	row int,
	viewport diffViewport,
	cursor diffCursor,
	selection *diffSelection,
) string {
	file := view.patch.Files[entry.file]
	line := file.Hunks[entry.hunk].Lines[entry.rightLine]
	gutter := lineGutter(line)
	width := max(20, viewport.Width)
	textWidth := width - lipgloss.Width(gutter)
	value := gutter + fitText(
		entry.text,
		textWidth,
		viewport.LeftColumn,
	)
	style := lineStyle(line.Kind, view.dark)
	colors := preserveForeground
	candidate := diffCursor{row: row, pane: cursor.pane}
	if selectionContainsCursor(selection, candidate) {
		style = selectionStyle(view.dark)
		colors = discardForeground
	}
	if cursor.row == row {
		style = cursorStyle
		colors = discardForeground
	}
	return renderStyledCell(value, width, 0, style, colors)
}

func lineGutter(line patches.Line) string {
	prefix := " "
	if line.Kind == patches.Addition {
		prefix = addedStyle.Render("+")
	}
	if line.Kind == patches.Deletion {
		prefix = removedStyle.Render("-")
	}
	oldNumber, newNumber := "", ""
	if line.OldNumber != 0 {
		oldNumber = strconv.Itoa(int(line.OldNumber))
	}
	if line.NewNumber != 0 {
		newNumber = strconv.Itoa(int(line.NewNumber))
	}
	return fmt.Sprintf("%5s %5s %s ", oldNumber, newNumber, prefix)
}

func lineStyle(kind patches.LineKind, dark bool) lipgloss.Style {
	lightDark := lipgloss.LightDark(dark)
	switch kind {
	case patches.Addition:
		return lipgloss.NewStyle().Background(lightDark(
			lipgloss.Color("#dafbe1"), lipgloss.Color("#1b3823"),
		))
	case patches.Deletion:
		return lipgloss.NewStyle().Background(lightDark(
			lipgloss.Color("#ffebe9"), lipgloss.Color("#402222"),
		))
	default:
		return contextStyle
	}
}

func selectionContainsCursor(selection *diffSelection,
	cursor diffCursor) bool {
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
	) && cursor.row <= max(
		first.row,
		last.row,
	)
}

func selectionStyle(dark bool) lipgloss.Style {
	return lipgloss.NewStyle().Background(lipgloss.LightDark(dark)(
		lipgloss.Color("#dbeafe"), lipgloss.Color("#1e3a5f"),
	))
}

func renderStyledCell(value string, width, offset int,
	style lipgloss.Style, colors foregroundPolicy) string {
	value = stripSGRColors(value, colors)
	fitted := fitText(value, width, offset)
	prefix := stylePrefix(style)
	if prefix != "" {
		fitted = strings.ReplaceAll(fitted, "\x1b[0m", "\x1b[0m"+prefix)
		fitted = strings.ReplaceAll(fitted, "\x1b[m", "\x1b[m"+prefix)
	}
	return style.Render(fitted)
}

func fitText(value string, width, offset int) string {
	if width <= 0 {
		return ""
	}
	value = strings.ReplaceAll(value, "\t", "    ")
	if offset > 0 {
		value = ansi.TruncateLeft(value, offset, "")
	}
	value = ansi.Truncate(value, width, "")
	if padding := width - lipgloss.Width(value); padding > 0 {
		value += strings.Repeat(" ", padding)
	}
	return value
}

func stripSGRColors(value string,
	policy foregroundPolicy) string {
	return ansiSGRPattern.ReplaceAllStringFunc(
		value,
		func(sequence string) string {
			parameters := sequence[2 : len(sequence)-1]
			if parameters == "" {
				return sequence
			}
			filtered := filteredSGR(parameters, policy)
			if filtered == "" {
				return ""
			}
			return "\x1b[" + filtered + "m"
		},
	)
}

func filteredSGR(parameters string,
	policy foregroundPolicy) string {
	parts := strings.Split(parameters, ";")
	filtered := make([]string, 0, len(parts))
	for index := 0; index < len(parts); index++ {
		parameter, next, keep := filteredSGRParameter(
			parts,
			index,
			policy,
		)
		if keep {
			filtered = append(filtered, parameter)
		}
		index = next
	}
	return strings.Join(filtered, ";")
}

func filteredSGRParameter(
	parts []string,
	index int,
	policy foregroundPolicy,
) (string, int, bool) {
	parameter := parts[index]
	code, err := strconv.Atoi(parameter)
	if err != nil {
		return parameter, index, true
	}
	if sGRHasColorParameters(code, policy) {
		return parameter, sGRColorEnd(parts, index), false
	}
	keep := !sGRColorCode(code) &&
		!(policy == discardForeground && sGRForegroundCode(
			code,
		))
	return parameter, index, keep
}

func sGRHasColorParameters(code int,
	policy foregroundPolicy) bool {
	return code == 48 || policy == discardForeground && code == 38
}

func sGRColorEnd(parts []string, index int) int {
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

func sGRColorCode(code int) bool {
	return code >= 40 && code <= 49 || code >= 100 && code <= 107
}

func sGRForegroundCode(code int) bool {
	return code >= 30 && code <= 39 || code >= 90 && code <= 97
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
