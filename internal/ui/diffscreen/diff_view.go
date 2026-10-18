package diffscreen

import (
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

type entry struct {
	kind              rowKind
	file, hunk        int
	leftLine          int
	rightLine         int
	text, left, right string
}

type diffView struct {
	patch        patch.Patch
	rows         []entry
	maxLineWidth int
	addedLines   int
	removedLines int
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
	v.appendNonLineRow(hunkRow, fileIndex, hunkIndex, hunkHeader(hunk.Header))
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
		if hunk.Lines[index].Kind == patch.Addition {
			appendRow(-1, index)
			index++
			continue
		}
		addedStart := changeBlockEnd(hunk.Lines, index, patch.Deletion)
		end := changeBlockEnd(hunk.Lines, addedStart, patch.Addition)
		appendPairedChangeRows(appendRow, index, addedStart, end)
		index = end
	}
}

func (v *diffView) appendLineRow(
	fileIndex, hunkIndex int,
	lines []patch.Line,
	highlighted highlightedSources,
	leftLine, rightLine int,
) {
	v.countSourceLine(lines, leftLine)
	if rightLine != leftLine {
		v.countSourceLine(lines, rightLine)
	}
	row := entry{
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

func (v *diffView) countSourceLine(lines []patch.Line, index int) {
	if index < 0 {
		return
	}
	switch lines[index].Kind {
	case patch.Addition:
		v.addedLines++
	case patch.Deletion:
		v.removedLines++
	default:
		return
	}
}

func (v *diffView) appendNonLineRow(
	kind rowKind,
	fileIndex, hunkIndex int,
	text string,
) {
	v.rows = append(v.rows, entry{
		kind:      kind,
		file:      fileIndex,
		hunk:      hunkIndex,
		leftLine:  -1,
		rightLine: -1,
		text:      text,
	})
}

func (v *diffView) highlightRow(
	row *entry,
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
		old: render(file.OldPath, file.OldSource, theme),
		new: render(file.NewPath, file.NewSource, theme),
	}
}

func hunkHeader(header string) string {
	if strings.HasPrefix(header, "@@") {
		return header
	}
	return "@@ " + header
}
