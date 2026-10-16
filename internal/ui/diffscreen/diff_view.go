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

// linePair stores source line indices shown in one display row. A negative
// index means that pane has no line for the row.
type linePair struct {
	leftLineIndex  int
	rightLineIndex int
}

type diffView struct {
	patch patch.Patch
	rows  []entry
	split bool
	dark  bool
}

func newDiffView(p patch.Patch, options Options) *diffView {
	v := &diffView{patch: p, split: options.SideBySide, dark: options.Dark}
	for fileIndex, file := range p.Files {
		v.appendFileRows(fileIndex, file)
	}
	return v
}

func (v *diffView) appendFileRows(fileIndex int, file patch.File) {
	v.rows = append(
		v.rows,
		entry{
			kind:      fileRow,
			file:      fileIndex,
			hunk:      -1,
			leftLine:  -1,
			rightLine: -1,
			text:      file.DisplayPath,
		},
	)
	for _, metadata := range file.Metadata {
		v.rows = append(
			v.rows,
			entry{
				kind:      metadataRow,
				file:      fileIndex,
				hunk:      -1,
				leftLine:  -1,
				rightLine: -1,
				text:      metadata,
			},
		)
	}
	highlighted := v.sourceHighlightsForFile(&file)
	for hunkIndex, hunk := range file.Hunks {
		v.appendHunkRows(fileIndex, hunkIndex, hunk, highlighted)
	}
}

func (v *diffView) appendHunkRows(
	fileIndex, hunkIndex int,
	hunk patch.Hunk,
	highlighted sourceHighlights,
) {
	v.rows = append(
		v.rows,
		entry{
			kind:      hunkRow,
			file:      fileIndex,
			hunk:      hunkIndex,
			leftLine:  -1,
			rightLine: -1,
			text:      formatHunkHeader(hunk.Header),
		},
	)
	for _, pair := range v.pairCorrespondingLineIndices(hunk.Lines) {
		row := entry{
			kind:      lineRow,
			file:      fileIndex,
			hunk:      hunkIndex,
			leftLine:  pair.leftLineIndex,
			rightLine: pair.rightLineIndex,
		}
		v.highlightRow(&row, hunk.Lines, highlighted)
		v.rows = append(v.rows, row)
	}
}

func (v *diffView) highlightRow(
	row *entry,
	lines []patch.Line,
	highlighted sourceHighlights,
) {
	if !v.split {
		row.text = highlightedTextForSourceLine(highlighted, lines[row.rightLine])
		return
	}
	if row.leftLine >= 0 {
		row.left = highlightedTextForSourceLine(highlighted, lines[row.leftLine])
	}
	if row.rightLine >= 0 {
		row.right = highlightedTextForSourceLine(highlighted, lines[row.rightLine])
	}
}

// pairCorrespondingLineIndices maps source lines to display rows. Split mode pairs each deletion
// block with the immediately following additions; unmatched panes use -1.
func (v *diffView) pairCorrespondingLineIndices(lines []patch.Line) []linePair {
	rows := make([]linePair, 0, len(lines))
	for index := 0; index < len(lines); {
		if !v.split || lines[index].Kind == patch.Context {
			rows = append(rows, linePair{
				leftLineIndex:  index,
				rightLineIndex: index,
			})
			index++
			continue
		}
		if lines[index].Kind == patch.Addition {
			rows = append(rows, linePair{
				leftLineIndex:  -1,
				rightLineIndex: index,
			})
			index++
			continue
		}
		addedStart := nextIndexAfterLineKindRun(lines, index, patch.Deletion)
		addedEnd := nextIndexAfterLineKindRun(lines, addedStart, patch.Addition)
		rows = append(
			rows,
			pairSideBySideChangeRows(index, addedStart, addedStart, addedEnd)...,
		)
		index = addedEnd
	}
	return rows
}

func nextIndexAfterLineKindRun(
	lines []patch.Line,
	startIndex int,
	kind patch.LineKind,
) (nextIndex int) {
	for startIndex < len(lines) && lines[startIndex].Kind == kind {
		startIndex++
	}
	return startIndex
}

// pairSideBySideChangeRows pairs source line indices from half-open ranges.
func pairSideBySideChangeRows(
	removedStart, removedEnd, addedStart, addedEnd int,
) []linePair {
	count := max(removedEnd-removedStart, addedEnd-addedStart)
	rows := make([]linePair, 0, count)
	for offset := 0; offset < count; offset++ {
		pair := linePair{leftLineIndex: -1, rightLineIndex: -1}
		if removedStart+offset < removedEnd {
			pair.leftLineIndex = removedStart + offset
		}
		if addedStart+offset < addedEnd {
			pair.rightLineIndex = addedStart + offset
		}
		rows = append(rows, pair)
	}
	return rows
}

func highlightedTextForSourceLine(highlighted sourceHighlights, line patch.Line) string {
	if line.Kind == patch.Deletion {
		return sourceLineOrFallback(highlighted.oldLines, line.OldNumber, line.Text)
	}
	return sourceLineOrFallback(highlighted.newLines, line.NewNumber, line.Text)
}

func (v *diffView) sourceHighlightsForFile(file *patch.File) sourceHighlights {
	theme := lightTheme
	if v.dark {
		theme = darkTheme
	}
	return sourceHighlights{
		oldLines: highlightSourceLines(file.OldPath, file.OldSource, theme),
		newLines: highlightSourceLines(file.NewPath, file.NewSource, theme),
	}
}

func formatHunkHeader(header string) string {
	if strings.HasPrefix(header, "@@") {
		return header
	}
	return "@@ " + header
}
