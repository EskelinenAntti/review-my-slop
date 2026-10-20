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
	countsLabel  string
	split        bool
	dark         bool
	styles       lineStyles
}

func (v *diffView) rowLine(current entry, pane diffPane) (patch.Line, bool) {
	index := v.lineIndex(current, pane)
	if index < 0 {
		return patch.Line{}, false
	}
	return v.patch.Files[current.file].Hunks[current.hunk].Lines[index], true
}

func newDiffView(p patch.Patch, options Options) *diffView {
	v := &diffView{
		patch:       p,
		split:       options.SideBySide,
		dark:        options.Dark,
		styles:      newLineStyles(options.Dark),
		countsLabel: changeCountsLabel(p),
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
	highlighted := v.highlight(file)
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
	pairs := unifiedLinePairs(hunk.Lines)
	if v.split {
		pairs = splitLinePairs(hunk.Lines)
	}
	for leftLine, rightLine := range pairs {
		v.appendLineRow(
			fileIndex,
			hunkIndex,
			hunk.Lines,
			highlighted,
			leftLine,
			rightLine,
		)
	}
}

func (v *diffView) appendLineRow(
	fileIndex, hunkIndex int,
	lines []patch.Line,
	highlighted highlightedSources,
	leftLine, rightLine int,
) {
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
		row.text = highlighted.text(lines[row.rightLine])
		return
	}
	if row.leftLine >= 0 {
		row.left = highlighted.text(lines[row.leftLine])
	}
	if row.rightLine >= 0 {
		row.right = highlighted.text(lines[row.rightLine])
	}
}

func normalizeHunkHeader(header string) string {
	if strings.HasPrefix(header, "@@") {
		return header
	}
	return "@@ " + header
}
