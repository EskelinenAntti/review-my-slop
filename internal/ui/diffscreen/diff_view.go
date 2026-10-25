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

type entry struct {
	kind       rowKind
	file, hunk int
	text       string
	panes      [2]paneContent
}

type paneContent struct {
	line int
	text string
}

type diffView struct {
	patch        patch.Patch
	rows         []entry
	maxLineWidth int
	countsLabel  string
	split        bool
	dark         bool
	styles       lineStyles
}

func newDiffView(p patch.Patch, options Options) *diffView {
	v := &diffView{
		patch:       p,
		countsLabel: changeCountsLabel(p),
		split:       options.SideBySide,
		dark:        options.Dark,
		styles:      newLineStyles(options.Dark),
	}
	for fileIndex, file := range p.Files {
		v.appendFileRows(fileIndex, file)
	}
	return v
}

func changeCountsLabel(p patch.Patch) string {
	var added, removed int
	for _, file := range p.Files {
		for _, hunk := range file.Hunks {
			for _, line := range hunk.Lines {
				switch line.Kind {
				case patch.Addition:
					added++
				case patch.Deletion:
					removed++
				default:
					continue
				}
			}
		}
	}
	return fmt.Sprintf("+%d-%d", added, removed)
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
			index = appendPairedChangeRows(appendRow, hunk.Lines, index)
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
	row := entry{
		kind:  lineRow,
		file:  fileIndex,
		hunk:  hunkIndex,
		panes: [2]paneContent{{line: leftLine}, {line: rightLine}},
	}
	v.highlightRow(&row, lines, highlighted)
	v.maxLineWidth = max(v.maxLineWidth, renderedLineWidth(row))
	v.rows = append(v.rows, row)
}

func (v *diffView) appendNonLineRow(
	kind rowKind,
	fileIndex, hunkIndex int,
	text string,
) {
	v.rows = append(v.rows, entry{
		kind:  kind,
		file:  fileIndex,
		hunk:  hunkIndex,
		panes: [2]paneContent{{line: -1}, {line: -1}},
		text:  text,
	})
}

func (v *diffView) highlightRow(
	row *entry,
	lines []patch.Line,
	highlighted highlightedSources,
) {
	if !v.split {
		pane := &row.panes[right]
		pane.text = sourceText(highlighted, lines[pane.line])
		return
	}
	for index := range row.panes {
		pane := &row.panes[index]
		if pane.line >= 0 {
			pane.text = sourceText(highlighted, lines[pane.line])
		}
	}
}

// appendPairedChangeRows pairs a deletion block with its immediately following
// additions and returns the first unconsumed line index.
func appendPairedChangeRows(
	appendRow func(leftLine, rightLine int),
	lines []patch.Line,
	removedStart int,
) int {
	addedStart := removedStart
	for addedStart < len(lines) && lines[addedStart].Kind == patch.Deletion {
		addedStart++
	}
	end := addedStart
	for end < len(lines) && lines[end].Kind == patch.Addition {
		end++
	}
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
	return end
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
