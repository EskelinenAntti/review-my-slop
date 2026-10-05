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
	kind                rowKind
	file, hunk          int
	leftLine, rightLine int
	text, left, right   string
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
		v.rows = append(v.rows, entry{kind: fileRow, file: fileIndex, hunk: -1, leftLine: -1, rightLine: -1, text: file.DisplayPath})
		for _, metadata := range file.Metadata {
			v.rows = append(v.rows, entry{kind: metadataRow, file: fileIndex, hunk: -1, leftLine: -1, rightLine: -1, text: metadata})
		}
		highlighted := pair{
			Old: render(file.OldPath, file.OldSource, dark),
			New: render(file.NewPath, file.NewSource, dark),
		}
		for hunkIndex, hunk := range file.Hunks {
			v.rows = append(v.rows, entry{kind: hunkRow, file: fileIndex, hunk: hunkIndex, leftLine: -1, rightLine: -1, text: hunkHeader(hunk.Header)})
			for _, indices := range lineRows(hunk.Lines, split) {
				row := entry{kind: lineRow, file: fileIndex, hunk: hunkIndex, leftLine: indices[0], rightLine: indices[1]}
				if !split {
					line := hunk.Lines[row.rightLine]
					row.text = sourceText(highlighted, line)
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
	}
	return v
}

// lineRows maps source lines to display rows. Split mode pairs each deletion
// block with the immediately following additions; unmatched panes use -1.
func lineRows(lines []patch.Line, split bool) [][2]int {
	rows := make([][2]int, 0, len(lines))
	for index := 0; index < len(lines); {
		if !split || lines[index].Kind == patch.Context {
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

func hunkHeader(header string) string {
	if strings.HasPrefix(header, "@@") {
		return header
	}
	return "@@ " + header
}
