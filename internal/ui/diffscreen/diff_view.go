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

func newDiffView(p patch.Patch, split, dark bool) *diffView {
	v := &diffView{patch: p, split: split, dark: dark}
	for fileIndex, file := range p.Files {
		v.appendFileRows(fileIndex, file)
	}
	return v
}

func (v *diffView) appendFileRows(fileIndex int, file patch.File) {
	v.rows = append(v.rows, entry{kind: fileRow, file: fileIndex, hunk: -1, leftLine: -1, rightLine: -1, text: file.DisplayPath})
	for _, metadata := range file.Metadata {
		v.rows = append(v.rows, entry{kind: metadataRow, file: fileIndex, hunk: -1, leftLine: -1, rightLine: -1, text: metadata})
	}
	highlighted := v.highlight(&file)
	for hunkIndex, hunk := range file.Hunks {
		v.appendHunkRows(fileIndex, hunkIndex, hunk, highlighted)
	}
}

func (v *diffView) appendHunkRows(fileIndex, hunkIndex int, hunk patch.Hunk, highlighted pair) {
	v.rows = append(v.rows, entry{kind: hunkRow, file: fileIndex, hunk: hunkIndex, leftLine: -1, rightLine: -1, text: hunkHeader(hunk.Header)})
	for _, indices := range lineRows(hunk.Lines, v.split) {
		row := entry{kind: lineRow, file: fileIndex, hunk: hunkIndex, leftLine: indices[0], rightLine: indices[1]}
		v.populateRowText(&row, hunk.Lines, highlighted)
		v.rows = append(v.rows, row)
	}
}

func (v *diffView) populateRowText(row *entry, lines []patch.Line, highlighted pair) {
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

// lineRows maps source lines to display rows. Split mode pairs each deletion
// block with the immediately following additions; unmatched panes use -1.
func lineRows(lines []patch.Line, split bool) [][2]int {
	rows := make([][2]int, 0, len(lines))
	for index := 0; index < len(lines); {
		rows, index = appendLineRowBlock(rows, lines, index, split)
	}
	return rows
}

func appendLineRowBlock(rows [][2]int, lines []patch.Line, index int, split bool) ([][2]int, int) {
	if !split || lines[index].Kind == patch.Context {
		return append(rows, [2]int{index, index}), index + 1
	}
	if lines[index].Kind == patch.Addition {
		return append(rows, [2]int{-1, index}), index + 1
	}
	addedStart := changeBlockEnd(lines, index, patch.Deletion)
	end := changeBlockEnd(lines, addedStart, patch.Addition)
	return appendPairedChangeRows(rows, index, addedStart, end), end
}

func changeBlockEnd(lines []patch.Line, index int, kind patch.LineKind) int {
	for index < len(lines) && lines[index].Kind == kind {
		index++
	}
	return index
}

func appendPairedChangeRows(rows [][2]int, removedStart, addedStart, end int) [][2]int {
	for offset := 0; offset < max(addedStart-removedStart, end-addedStart); offset++ {
		rows = append(rows, [2]int{blockLineIndex(removedStart+offset, addedStart), blockLineIndex(addedStart+offset, end)})
	}
	return rows
}

func blockLineIndex(index, end int) int {
	if index < end {
		return index
	}
	return -1
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
