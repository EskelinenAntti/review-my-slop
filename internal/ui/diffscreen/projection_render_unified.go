package diffscreen

import (
	"fmt"

	"charm.land/lipgloss/v2"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

func (r rowRenderer) renderUnifiedRow(current displayRow, y int) string {
	width := max(20, r.viewport.Width)
	switch current.kind {
	case fileRow:
		return renderFileHeaderRow(current.text, width)
	case metadataRow:
		return metadataStyle.Render("  " + current.text)
	case hunkRow:
		return hunkStyle.Render(current.text)
	case lineRow:
		return r.renderUnifiedLine(current, y)
	}
	return ""
}

func (r rowRenderer) renderUnifiedLine(current displayRow, y int) string {
	width := max(20, r.viewport.Width)
	hunk := r.projection.patch.Files[current.file].Hunks[current.hunk]
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
		formatLineNumber(line.OldNumber),
		formatLineNumber(line.NewNumber),
		prefix,
	)
	value := gutter + fitANSIWindow(
		current.text,
		r.viewport.LeftColumn,
		width-lipgloss.Width(gutter),
	)
	style := lineStyle(line.Kind, r.projection.dark)
	colors := preserveForeground
	candidate := patchCursor{row: y, pane: r.cursor.pane}
	if cursorIsSelected(r.selection, candidate) {
		style, colors = selectionRowStyle(r.projection.dark), discardForeground
	}
	if r.cursor.row == y {
		style, colors = cursorStyle, discardForeground
	}
	return renderStyledRow(style, value, width, colors)
}

func renderFileHeaderRow(path string, width int) string {
	return fileStyle.Width(max(20, width)).Render(path)
}
