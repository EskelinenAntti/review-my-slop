package diffscreen

import (
	"fmt"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/ui/internal/frame"
)

type diffRenderer struct {
	view      *diffView
	viewport  diffViewport
	cursor    diffCursor
	selection *diffSelection
}

func (v *diffView) render(
	viewport diffViewport,
	cursor diffCursor,
	selection *diffSelection,
) []string {
	viewport = v.clampViewport(viewport)
	renderer := diffRenderer{
		view:      v,
		viewport:  viewport,
		cursor:    cursor,
		selection: selection,
	}
	lines := make([]string, 0, viewport.height)
	if v.hasStickyHeader(viewport.top, viewport.height) {
		current := v.rows[viewport.top]
		lines = append(
			lines,
			renderFileRow(
				v.patch.Files[current.file].DisplayPath,
				viewport.width,
			),
		)
	}
	end := min(len(v.rows), viewport.top+v.contentHeight(viewport))
	for y := viewport.top; y < end; y++ {
		lines = append(lines, renderer.renderRow(y))
	}
	for len(lines) < viewport.height {
		lines = append(lines, "")
	}
	return lines
}

func (r diffRenderer) renderRow(y int) string {
	current := r.view.rows[y]
	width := frame.ScreenWidth(r.viewport.width)
	switch current.kind {
	case fileRow:
		return renderFileRow(current.text, width)
	case metadataRow:
		return metadataStyle.Render("  " + current.text)
	case hunkRow:
		return hunkStyle.Render(current.text)
	case lineRow:
		if r.view.split {
			return r.renderSplitRow(y)
		}
		return r.renderLine(diffCursor{row: y, pane: r.cursor.pane}, width)
	}
	return ""
}

func renderFileRow(path string, width int) string {
	return fileStyle.Width(frame.ScreenWidth(width)).Render(path)
}

func (r diffRenderer) renderSplitRow(y int) string {
	leftWidth, rightWidth := splitPaneWidths(r.viewport.width)
	left := r.renderLine(diffCursor{row: y, pane: left}, leftWidth)
	right := r.renderLine(diffCursor{row: y, pane: right}, rightWidth)
	return left + " │ " + right
}

func splitPaneWidths(width int) (left, right int) {
	left = max(20, (width-3)/2)
	return left, max(20, width-3-left)
}

func (r diffRenderer) renderLine(candidate diffCursor, width int) string {
	line, ok := r.view.line(candidate)
	if !ok {
		return strings.Repeat(" ", width)
	}
	current := r.view.rows[candidate.row]
	marker := lineMarker(line.Kind)
	var gutter, text string
	if r.view.split {
		text = current.right
		numberValue := line.NewNumber
		if candidate.pane == left {
			text, numberValue = current.left, line.OldNumber
		}
		text = marker + " " + text
		gutter = fmt.Sprintf("%5s ", number(numberValue))
	} else {
		text = current.text
		gutter = fmt.Sprintf(
			"%5s %5s %s ",
			number(line.OldNumber),
			number(line.NewNumber),
			marker,
		)
	}
	return r.renderCodeLine(
		gutter,
		text,
		width,
		line,
		candidate,
	)
}

func (r diffRenderer) renderCodeLine(
	gutter, text string,
	width int,
	line patch.Line,
	candidate diffCursor,
) string {
	value := gutter + fitANSIWindow(
		text,
		r.viewport.leftColumn,
		width-lipgloss.Width(gutter),
	)
	style, colors := r.lineStyle(line.Kind, candidate)
	return renderStyledRow(style, value, width, colors)
}

func number(value patch.LineNumber) string {
	if value == 0 {
		return ""
	}
	return strconv.Itoa(int(value))
}
