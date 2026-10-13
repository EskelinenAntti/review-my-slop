package diffscreen

import (
	"fmt"

	"charm.land/lipgloss/v2"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

func (r diffRenderer) renderUnifiedRow(current entry, y int) string {
	width := max(20, r.viewport.Width)
	switch current.kind {
	case fileRow:
		return renderFileRow(current.text, width)
	case metadataRow:
		return metadataStyle.Render("  " + current.text)
	case hunkRow:
		return hunkStyle.Render(current.text)
	case lineRow:
		return r.renderUnifiedLine(current, y)
	}
	return ""
}

func (r diffRenderer) renderUnifiedLine(current entry, y int) string {
	width := max(20, r.viewport.Width)
	hunk := r.view.patch.Files[current.file].Hunks[current.hunk]
	line := hunk.Lines[current.rightLine]
	prefix := " "
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
	value := gutter + fitANSIWindow(
		current.text,
		r.viewport.LeftColumn,
		width-lipgloss.Width(gutter),
	)
	style := lineStyle(line.Kind, r.view.dark)
	colors := preserveForeground
	candidate := diffCursor{row: y, pane: r.cursor.pane}
	if selected(r.selection, candidate) {
		style, colors = selectionRowStyle(r.view.dark), discardForeground
	}
	if r.cursor.row == y {
		style, colors = cursorStyle, discardForeground
	}
	return renderStyledRow(style, value, width, colors)
}

func renderFileRow(path string, width int) string {
	return fileStyle.Width(max(20, width)).Render(path)
}
