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

// rowProjection builds display rows independently of interactive view state.
type rowProjection struct {
	rows         []entry
	maxLineWidth int
	options      Options
}

func projectRows(p patch.Patch, options Options) ([]entry, int) {
	projection := rowProjection{options: options}
	for fileIndex, file := range p.Files {
		projection.appendFileRows(fileIndex, file)
	}
	return projection.rows, projection.maxLineWidth
}

func (p *rowProjection) appendFileRows(fileIndex int, file patch.File) {
	p.appendNonLineRow(fileRow, fileIndex, -1, file.DisplayPath)
	for _, metadata := range file.Metadata {
		p.appendNonLineRow(metadataRow, fileIndex, -1, metadata)
	}
	highlighted := highlightFile(file, p.options)
	for hunkIndex, hunk := range file.Hunks {
		p.appendHunkRows(fileIndex, hunkIndex, hunk, highlighted)
	}
}

func (p *rowProjection) appendHunkRows(
	fileIndex, hunkIndex int,
	hunk patch.Hunk,
	highlighted highlightedSources,
) {
	p.appendNonLineRow(
		hunkRow,
		fileIndex,
		hunkIndex,
		normalizeHunkHeader(hunk.Header),
	)
	pairs := unifiedLinePairs
	if p.options.SideBySide {
		pairs = splitLinePairs
	}
	for leftLine, rightLine := range pairs(hunk.Lines) {
		row := entry{
			kind:      lineRow,
			file:      fileIndex,
			hunk:      hunkIndex,
			leftLine:  leftLine,
			rightLine: rightLine,
		}
		p.highlightRow(&row, hunk.Lines, highlighted)
		p.maxLineWidth = max(p.maxLineWidth, renderedLineWidth(row))
		p.rows = append(p.rows, row)
	}
}

func (p *rowProjection) appendNonLineRow(
	kind rowKind,
	fileIndex, hunkIndex int,
	text string,
) {
	p.rows = append(p.rows, entry{
		kind:      kind,
		file:      fileIndex,
		hunk:      hunkIndex,
		leftLine:  -1,
		rightLine: -1,
		text:      text,
	})
}

func (p *rowProjection) highlightRow(
	row *entry,
	lines []patch.Line,
	highlighted highlightedSources,
) {
	if !p.options.SideBySide {
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
