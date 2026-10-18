package diffscreen

import (
	"fmt"
	"strings"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

type rowKind uint8

const (
	fileRow rowKind = iota
	metadataRow
	hunkRow
	lineRow
)

// diffRow maps a display row to source indices within a file and hunk.
// Negative line indices denote empty panes or non-code rows.
type diffRow struct {
	kind              rowKind
	file, hunk        int
	leftLine          int
	rightLine         int
	text, left, right string
}

type diffView struct {
	patch        patch.Patch
	rows         []diffRow
	maxLineWidth int
	addedLines   int
	removedLines int
	countsLabel  string
	split        bool
	dark         bool
	styles       lineStyles
}

func newDiffView(p patch.Patch, options Options) *diffView {
	v := &diffView{
		patch:  p,
		split:  options.SideBySide,
		dark:   options.Dark,
		styles: newLineStyles(options.Dark),
	}
	for fileIndex, file := range p.Files {
		v.appendFileRows(fileIndex, file)
	}
	v.countsLabel = fmt.Sprintf("+%d-%d", v.addedLines, v.removedLines)
	return v
}

func (v *diffView) appendFileRows(fileIndex int, file patch.File) {
	v.appendNonLineRow(fileRow, fileIndex, -1, file.DisplayPath)
	for _, metadata := range file.Metadata {
		v.appendNonLineRow(metadataRow, fileIndex, -1, metadata)
	}
	highlighted := v.highlight(&file)
	for hunkIndex, hunk := range file.Hunks {
		v.appendHunkRows(fileIndex, hunkIndex, hunk, highlighted)
	}
}

func (v *diffView) appendHunkRows(
	fileIndex, hunkIndex int,
	hunk patch.Hunk,
	highlighted highlightedSources,
) {
	v.countSourceLines(hunk.Lines)
	v.appendNonLineRow(
		hunkRow,
		fileIndex,
		hunkIndex,
		normalizeHunkHeader(hunk.Header),
	)
	appendRow := func(leftLine, rightLine int) {
		v.appendLineRow(
			fileIndex,
			hunkIndex,
			hunk.Lines,
			highlighted,
			leftLine,
			rightLine,
		)
	}
	for index := 0; index < len(hunk.Lines); {
		if !v.split || hunk.Lines[index].Kind == patch.Context {
			appendRow(index, index)
			index++
			continue
		}
		switch hunk.Lines[index].Kind {
		case patch.Addition:
			appendRow(-1, index)
			index++
		case patch.Deletion:
			addedStart := changeBlockEnd(hunk.Lines, index, patch.Deletion)
			end := changeBlockEnd(hunk.Lines, addedStart, patch.Addition)
			appendPairedChangeRows(appendRow, index, addedStart, end)
			index = end
		default:
			appendRow(index, index)
			index++
		}
	}
}

func (v *diffView) appendLineRow(
	fileIndex, hunkIndex int,
	lines []patch.Line,
	highlighted highlightedSources,
	leftLine, rightLine int,
) {
	row := diffRow{
		kind:      lineRow,
		file:      fileIndex,
		hunk:      hunkIndex,
		leftLine:  leftLine,
		rightLine: rightLine,
	}
	v.highlightRow(&row, lines, highlighted)
	v.maxLineWidth = max(v.maxLineWidth, renderedLineWidth(row))
	v.rows = append(v.rows, row)
}

func (v *diffView) countSourceLines(lines []patch.Line) {
	for _, line := range lines {
		switch line.Kind {
		case patch.Addition:
			v.addedLines++
		case patch.Deletion:
			v.removedLines++
		default:
			continue
		}
	}
}

func (v *diffView) appendNonLineRow(
	kind rowKind,
	fileIndex, hunkIndex int,
	text string,
) {
	v.rows = append(v.rows, diffRow{
		kind:      kind,
		file:      fileIndex,
		hunk:      hunkIndex,
		leftLine:  -1,
		rightLine: -1,
		text:      text,
	})
}

func (v *diffView) highlightRow(
	row *diffRow,
	lines []patch.Line,
	highlighted highlightedSources,
) {
	if !v.split {
		row.text = sourceText(highlighted, lines[row.rightLine])
		return
	}
	if row.leftLine >= 0 {
		row.left = sourceText(highlighted, lines[row.leftLine])
	}
	if row.rightLine >= 0 {
		row.right = sourceText(highlighted, lines[row.rightLine])
	}
}

func changeBlockEnd(lines []patch.Line, start int, kind patch.LineKind) int {
	for start < len(lines) && lines[start].Kind == kind {
		start++
	}
	return start
}

func appendPairedChangeRows(
	appendRow func(leftLine, rightLine int),
	removedStart, addedStart, end int,
) {
	count := max(addedStart-removedStart, end-addedStart)
	for offset := 0; offset < count; offset++ {
		leftLine, rightLine := -1, -1
		if removedStart+offset < addedStart {
			leftLine = removedStart + offset
		}
		if addedStart+offset < end {
			rightLine = addedStart + offset
		}
		appendRow(leftLine, rightLine)
	}
}

func sourceText(highlighted highlightedSources, line patch.Line) string {
	if line.Kind == patch.Deletion {
		return highlightedLine(highlighted.old, line.OldNumber, line.Text)
	}
	return highlightedLine(highlighted.new, line.NewNumber, line.Text)
}

func (v *diffView) highlight(file *patch.File) highlightedSources {
	theme := lightTheme
	if v.dark {
		theme = darkTheme
	}
	return highlightedSources{
		old: highlightSource(file.OldPath, file.OldSource, theme),
		new: highlightSource(file.NewPath, file.NewSource, theme),
	}
}

func normalizeHunkHeader(header string) string {
	if strings.HasPrefix(header, "@@") {
		return header
	}
	return "@@ " + header
}
