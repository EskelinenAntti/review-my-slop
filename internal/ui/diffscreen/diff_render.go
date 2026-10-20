package diffscreen

import (
	"fmt"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/ui/internal/frame"
)

func (v *diffView) render(
	viewport diffViewport,
	cursor diffCursor,
	selection *diffSelection,
) string {
	viewport = v.clampViewport(viewport)
	layout := newSplitLayout(viewport.width)
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
		lines = append(lines,
			v.renderRow(y, viewport, layout, cursor, selection),
		)
	}
	for len(lines) < viewport.height {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func (v *diffView) renderRow(
	y int,
	viewport diffViewport,
	layout splitLayout,
	cursor diffCursor,
	selection *diffSelection,
) string {
	current := v.rows[y]
	switch current.kind {
	case fileRow:
		return renderFileRow(current.text, viewport.width)
	case metadataRow:
		return metadataStyle.Render("  " + current.text)
	case hunkRow:
		return hunkStyle.Render(current.text)
	case lineRow:
		if v.split {
			return v.renderSplitRow(
				current, y, viewport, layout, cursor, selection,
			)
		}
		return v.renderUnifiedLine(
			current,
			y,
			viewport,
			cursor,
			selection,
		)
	}
	return ""
}

func (v *diffView) renderUnifiedLine(
	current entry,
	y int,
	viewport diffViewport,
	cursor diffCursor,
	selection *diffSelection,
) string {
	width := frame.ScreenWidth(viewport.width)
	line, _ := v.rowLine(current, right)
	prefix := lineMarker(line.Kind)
	gutter := fmt.Sprintf(
		"%5s %5s %s ",
		number(line.OldNumber),
		number(line.NewNumber),
		prefix,
	)
	value := gutter + fitANSIWindow(
		current.text,
		viewport.leftColumn,
		width-lipgloss.Width(gutter),
	)
	return v.renderCodeLine(
		value,
		width,
		line,
		diffCursor{row: y, pane: cursor.pane},
		cursor,
		selection,
	)
}

func renderFileRow(path string, width int) string {
	return fileStyle.Width(frame.ScreenWidth(width)).Render(path)
}

func (v *diffView) renderSplitRow(
	current entry,
	y int,
	viewport diffViewport,
	layout splitLayout,
	cursor diffCursor,
	selection *diffSelection,
) string {
	left := v.renderPane(
		current,
		y,
		left,
		layout.leftWidth,
		viewport.leftColumn,
		cursor,
		selection,
	)
	right := v.renderPane(
		current,
		y,
		right,
		layout.rightWidth,
		viewport.leftColumn,
		cursor,
		selection,
	)
	return left + splitPaneDivider + right
}

func (v *diffView) renderPane(
	current entry,
	y int,
	pane diffPane,
	width, offset int,
	cursor diffCursor,
	selection *diffSelection,
) string {
	line, ok := v.rowLine(current, pane)
	if !ok {
		return strings.Repeat(" ", width)
	}
	text := current.right
	numberValue := line.NewNumber
	if pane == left {
		text, numberValue = current.left, line.OldNumber
	}
	prefix := lineMarker(line.Kind) + " "
	gutter := fmt.Sprintf("%5s ", number(numberValue))
	value := gutter + fitANSIWindow(
		prefix+text,
		offset,
		width-lipgloss.Width(gutter),
	)
	return v.renderCodeLine(
		value,
		width,
		line,
		diffCursor{row: y, pane: pane},
		cursor,
		selection,
	)
}

func (v *diffView) renderCodeLine(
	value string,
	width int,
	line patch.Line,
	candidate, cursor diffCursor,
	selection *diffSelection,
) string {
	style, colors := renderedLineStyle(
		line.Kind,
		candidate,
		cursor,
		selection,
		v.styles,
	)
	return renderStyledRow(style, value, width, colors)
}

func number(value patch.LineNumber) string {
	if value == 0 {
		return ""
	}
	return strconv.Itoa(int(value))
}
