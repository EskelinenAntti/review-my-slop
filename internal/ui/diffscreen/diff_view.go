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
		headingEntry(fileRow, fileIndex, -one, file.DisplayPath),
	)
	for _, metadata := range file.Metadata {
		v.rows = append(
			v.rows,
			headingEntry(metadataRow, fileIndex, -one, metadata),
		)
	}
	highlighted := v.highlight(&file)
	for hunkIndex, hunk := range file.Hunks {
		v.rows = append(
			v.rows,
			headingEntry(
				hunkRow,
				fileIndex,
				hunkIndex,
				hunkHeader(hunk.Header),
			),
		)
		v.appendHunkRows(fileIndex, hunkIndex, hunk, highlighted)
	}
}

func headingEntry(kind rowKind, file, hunk int, text string) entry {
	return entry{
		kind:      kind,
		file:      file,
		hunk:      hunk,
		leftLine:  -one,
		rightLine: -one,
		text:      text,
	}
}

func (v *diffView) appendHunkRows(
	fileIndex, hunkIndex int,
	hunk patch.Hunk,
	highlighted pair,
) {
	for _, indices := range lineRows(hunk.Lines, Options{SideBySide: v.split}) {
		row := entry{
			kind:      lineRow,
			file:      fileIndex,
			hunk:      hunkIndex,
			leftLine:  indices[zero],
			rightLine: indices[one],
		}
		v.rows = append(v.rows, v.highlightRow(row, hunk.Lines, highlighted))
	}
}

func (v *diffView) highlightRow(
	row entry,
	lines []patch.Line,
	highlighted pair,
) entry {
	if !v.split {
		row.text = sourceText(highlighted, lines[row.rightLine])
		return row
	}
	if row.leftLine >= zero {
		row.left = sourceText(highlighted, lines[row.leftLine])
	}
	if row.rightLine >= zero {
		row.right = sourceText(highlighted, lines[row.rightLine])
	}
	return row
}

// lineRows maps source lines to display rows. Split mode pairs each deletion
// block with the immediately following additions; unmatched panes use -1.
func lineRows(lines []patch.Line, options Options) [][two]int {
	if !options.SideBySide {
		rows := make([][two]int, len(lines))
		for index := range lines {
			rows[index] = [two]int{index, index}
		}
		return rows
	}
	return splitLineRows(lines)
}

func splitLineRows(lines []patch.Line) [][two]int {
	rows := make([][two]int, zero, len(lines))
	for index := zero; index < len(lines); {
		switch lines[index].Kind {
		case patch.Context:
			rows = append(rows, [two]int{index, index})
			index++
		case patch.Addition:
			rows = append(rows, [two]int{-one, index})
			index++
		default:
			removedStart := index
			addedStart := changeBlockEnd(lines, index, patch.Deletion)
			index = changeBlockEnd(lines, addedStart, patch.Addition)
			rows = append(
				rows,
				pairedChangeRows(removedStart, addedStart, index)...,
			)
		}
	}
	return rows
}

func changeBlockEnd(lines []patch.Line, start int, kind patch.LineKind) int {
	end := start
	for end < len(lines) && lines[end].Kind == kind {
		end++
	}
	return end
}

func pairedChangeRows(removedStart, addedStart, end int) [][two]int {
	rows := make(
		[][two]int,
		zero,
		max(addedStart-removedStart, end-addedStart),
	)
	for offset := zero; offset < cap(rows); offset++ {
		indices := [two]int{-one, -one}
		if removedStart+offset < addedStart {
			indices[zero] = removedStart + offset
		}
		if addedStart+offset < end {
			indices[one] = addedStart + offset
		}
		rows = append(rows, indices)
	}
	return rows
}

func sourceText(highlighted pair, line patch.Line) string {
	if line.Kind == patch.Deletion {
		return highlightedLine(highlighted.Old, line.OldNumber, line.Text)
	}
	return highlightedLine(highlighted.New, line.NewNumber, line.Text)
}

func (v *diffView) highlight(file *patch.File) pair {
	return pair{
		Old: render(file.OldPath, file.OldSource, v.dark),
		New: render(file.NewPath, file.NewSource, v.dark),
	}
}

func hunkHeader(header string) string {
	if strings.HasPrefix(header, "@@") {
		return header
	}
	return "@@ " + header
}
