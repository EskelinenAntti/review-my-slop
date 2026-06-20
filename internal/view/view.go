package view

import (
	"strings"

	"github.com/eskelinenantti/review-my-slop/internal/highlight"
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

func NewUnifiedView(p patch.Patch, dark bool) View {
	v := &diffView{patch: p, dark: dark}
	v.buildUnified()
	return v
}

func NewSplitView(p patch.Patch, dark bool) View {
	v := &diffView{patch: p, split: true, dark: dark}
	v.buildSplit()
	return v
}

func (v *diffView) buildUnified() {
	for fileIndex := range v.patch.Files {
		file := &v.patch.Files[fileIndex]
		v.rows = append(v.rows, entry{kind: fileRow, file: fileIndex, hunk: -1, leftLine: -1, rightLine: -1, text: file.DisplayPath})
		for _, metadata := range file.Metadata {
			v.rows = append(v.rows, entry{kind: metadataRow, file: fileIndex, hunk: -1, leftLine: -1, rightLine: -1, text: metadata})
		}
		highlighted := highlight.Sources(file.Path(), file.OldSource, file.NewSource, v.dark)
		for hunkIndex := range file.Hunks {
			hunk := &file.Hunks[hunkIndex]
			v.rows = append(v.rows, entry{kind: hunkRow, file: fileIndex, hunk: hunkIndex, leftLine: -1, rightLine: -1, text: hunkHeader(hunk.Header)})
			for lineIndex, line := range hunk.Lines {
				text := line.Text
				if line.Kind == patch.Deletion {
					text = highlightedLine(highlighted.Old, line.OldNumber, text)
				} else {
					text = highlightedLine(highlighted.New, line.NewNumber, text)
				}
				v.rows = append(v.rows, entry{kind: lineRow, file: fileIndex, hunk: hunkIndex, leftLine: lineIndex, rightLine: lineIndex, text: text})
			}
		}
	}
}

func (v *diffView) buildSplit() {
	for fileIndex := range v.patch.Files {
		file := &v.patch.Files[fileIndex]
		v.rows = append(v.rows, entry{kind: fileRow, file: fileIndex, hunk: -1, leftLine: -1, rightLine: -1, text: file.DisplayPath})
		for _, metadata := range file.Metadata {
			v.rows = append(v.rows, entry{kind: metadataRow, file: fileIndex, hunk: -1, leftLine: -1, rightLine: -1, text: metadata})
		}
		highlighted := highlight.Sources(file.Path(), file.OldSource, file.NewSource, v.dark)
		for hunkIndex := range file.Hunks {
			hunk := &file.Hunks[hunkIndex]
			v.rows = append(v.rows, entry{kind: hunkRow, file: fileIndex, hunk: hunkIndex, leftLine: -1, rightLine: -1, text: hunkHeader(hunk.Header)})
			for index := 0; index < len(hunk.Lines); {
				line := hunk.Lines[index]
				switch line.Kind {
				case patch.Context:
					text := highlightedLine(highlighted.New, line.NewNumber, line.Text)
					v.rows = append(v.rows, entry{kind: lineRow, file: fileIndex, hunk: hunkIndex, leftLine: index, rightLine: index, left: text, right: text})
					index++
				case patch.Addition:
					text := highlightedLine(highlighted.New, line.NewNumber, line.Text)
					v.rows = append(v.rows, entry{kind: lineRow, file: fileIndex, hunk: hunkIndex, leftLine: -1, rightLine: index, right: text})
					index++
				case patch.Deletion:
					removedStart := index
					for index < len(hunk.Lines) && hunk.Lines[index].Kind == patch.Deletion {
						index++
					}
					addedStart, addedEnd := index, index
					for addedEnd < len(hunk.Lines) && hunk.Lines[addedEnd].Kind == patch.Addition {
						addedEnd++
					}
					count := max(index-removedStart, addedEnd-addedStart)
					for offset := 0; offset < count; offset++ {
						current := entry{kind: lineRow, file: fileIndex, hunk: hunkIndex, leftLine: -1, rightLine: -1}
						if removedStart+offset < index {
							current.leftLine = removedStart + offset
							old := hunk.Lines[current.leftLine]
							current.left = highlightedLine(highlighted.Old, old.OldNumber, old.Text)
						}
						if addedStart+offset < addedEnd {
							current.rightLine = addedStart + offset
							added := hunk.Lines[current.rightLine]
							current.right = highlightedLine(highlighted.New, added.NewNumber, added.Text)
						}
						v.rows = append(v.rows, current)
					}
					index = addedEnd
				}
			}
		}
	}
}

func hunkHeader(header string) string {
	if strings.HasPrefix(header, "@@") {
		return header
	}
	return "@@ " + header
}
