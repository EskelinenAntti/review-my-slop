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

type displayRow struct {
	kind              rowKind
	file, hunk        int
	leftLine          int
	rightLine         int
	text, left, right string
}

type projection struct {
	patch patch.Patch
	rows  []displayRow
	split bool
	dark  bool
}

func buildProjection(p patch.Patch, split, dark bool) *projection {
	v := &projection{patch: p, split: split, dark: dark}
	for fileIndex, file := range p.Files {
		v.rows = append(v.rows, displayRow{kind: fileRow, file: fileIndex, hunk: -1, leftLine: -1, rightLine: -1, text: file.DisplayPath})
		for _, metadata := range file.Metadata {
			v.rows = append(v.rows, displayRow{kind: metadataRow, file: fileIndex, hunk: -1, leftLine: -1, rightLine: -1, text: metadata})
		}
		highlighted := highlightFile(&file, dark)
		for hunkIndex, hunk := range file.Hunks {
			v.rows = append(v.rows, displayRow{kind: hunkRow, file: fileIndex, hunk: hunkIndex, leftLine: -1, rightLine: -1, text: formatHunkHeader(hunk.Header)})
			for _, indices := range projectHunkLines(hunk.Lines, split) {
				row := displayRow{kind: lineRow, file: fileIndex, hunk: hunkIndex, leftLine: indices[left], rightLine: indices[right]}
				if !split {
					line := hunk.Lines[row.rightLine]
					row.text = highlighted.textFor(line)
				} else {
					if row.leftLine >= 0 {
						row.left = highlighted.textFor(hunk.Lines[row.leftLine])
					}
					if row.rightLine >= 0 {
						row.right = highlighted.textFor(hunk.Lines[row.rightLine])
					}
				}
				v.rows = append(v.rows, row)
			}
		}
	}
	return v
}

// projectHunkLines maps source lines to display rows. Split mode pairs each deletion
// block with the immediately following additions; unmatched panes use -1.
func projectHunkLines(lines []patch.Line, split bool) [][2]int {
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
				pair[left] = removedStart + offset
			}
			if addedStart+offset < index {
				pair[right] = addedStart + offset
			}
			rows = append(rows, pair)
		}
	}
	return rows
}

func formatHunkHeader(header string) string {
	if strings.HasPrefix(header, "@@") {
		return header
	}
	return "@@ " + header
}
