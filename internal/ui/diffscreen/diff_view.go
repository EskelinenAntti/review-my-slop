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
	for _, indices := range v.lineRows(hunk.Lines) {
		row := entry{kind: lineRow, file: fileIndex, hunk: hunkIndex, leftLine: indices[0], rightLine: indices[1]}
		if !v.split {
			row.text = sourceText(highlighted, hunk.Lines[row.rightLine])
		} else {
			if row.leftLine >= 0 {
				row.left = sourceText(highlighted, hunk.Lines[row.leftLine])
			}
			if row.rightLine >= 0 {
				row.right = sourceText(highlighted, hunk.Lines[row.rightLine])
			}
		}
		v.rows = append(v.rows, row)
	}
}

// lineRows maps source lines to display rows. Split mode pairs each deletion
// block with the immediately following additions; unmatched panes use -1.
func (v *diffView) lineRows(lines []patch.Line) [][2]int {
	rows := make([][2]int, 0, len(lines))
	for index := 0; index < len(lines); {
		if !v.split || lines[index].Kind == patch.Context {
			rows = append(rows, [2]int{index, index})
			index++
			continue
		}
		if lines[index].Kind == patch.Addition {
			rows = append(rows, [2]int{-1, index})
			index++
			continue
		}
		removedStart := index
		for index < len(lines) && lines[index].Kind == patch.Deletion {
			index++
		}
		addedStart := index
		for index < len(lines) && lines[index].Kind == patch.Addition {
			index++
		}
		for offset := 0; offset < max(addedStart-removedStart, index-addedStart); offset++ {
			pair := [2]int{-1, -1}
			if removedStart+offset < addedStart {
				pair[0] = removedStart + offset
			}
			if addedStart+offset < index {
				pair[1] = addedStart + offset
			}
			rows = append(rows, pair)
		}
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
	theme := lightTheme
	if v.dark {
		theme = darkTheme
	}
	return pair{
		Old: render(file.OldPath, file.OldSource, theme),
		New: render(file.NewPath, file.NewSource, theme),
	}
}

func hunkHeader(header string) string {
	if strings.HasPrefix(header, "@@") {
		return header
	}
	return "@@ " + header
}
