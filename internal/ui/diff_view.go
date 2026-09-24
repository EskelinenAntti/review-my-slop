package ui

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

func newEntry(kind rowKind, file, hunk, leftLine, rightLine int, text, left, right string) entry {
	return entry{kind, file, hunk, leftLine, rightLine, text, left, right}
}

func NewUnifiedView(p patch.Patch, dark bool) View {
	v := &diffView{patch: p, dark: dark}
	v.build()
	return v
}

func NewSideBySideView(p patch.Patch, dark bool) View {
	v := &diffView{patch: p, split: true, dark: dark}
	v.build()
	return v
}

func (v *diffView) build() {
	for fileIndex := range v.patch.Files {
		file := &v.patch.Files[fileIndex]
		v.appendFileHeader(fileIndex, file)
		highlighted := Sources(file.Path(), file.OldSource, file.NewSource, v.dark)
		for hunkIndex := range file.Hunks {
			hunk := &file.Hunks[hunkIndex]
			header := hunk.Header
			if !strings.HasPrefix(header, "@@") {
				header = "@@ " + header
			}
			v.rows = append(v.rows, newEntry(hunkRow, fileIndex, hunkIndex, -1, -1, header, "", ""))
			if v.split {
				v.appendSplitLines(fileIndex, hunkIndex, hunk, highlighted)
				continue
			}
			for lineIndex, line := range hunk.Lines {
				text := line.Text
				if line.Kind == patch.Deletion {
					text = highlightedLine(highlighted.Old, line.OldNumber, text)
				} else {
					text = highlightedLine(highlighted.New, line.NewNumber, text)
				}
				v.rows = append(v.rows, newEntry(lineRow, fileIndex, hunkIndex, lineIndex, lineIndex, text, "", ""))
			}
		}
	}
}

func (v *diffView) appendSplitLines(fileIndex, hunkIndex int, hunk *patch.Hunk, highlighted Pair) {
	lines := hunk.Lines
	base := newEntry(lineRow, fileIndex, hunkIndex, -1, -1, "", "", "")
	for index := 0; index < len(lines); {
		line := lines[index]
		switch line.Kind {
		case patch.Context, patch.Addition:
			text := highlightedLine(highlighted.New, line.NewNumber, line.Text)
			current := base
			current.rightLine, current.right = index, text
			if line.Kind == patch.Context {
				current.leftLine, current.left = index, text
			}
			v.rows = append(v.rows, current)
			index++
		case patch.Deletion:
			removedStart := index
			for index < len(lines) && lines[index].Kind == patch.Deletion {
				index++
			}
			addedStart, addedEnd := index, index
			for addedEnd < len(lines) && lines[addedEnd].Kind == patch.Addition {
				addedEnd++
			}
			count := max(index-removedStart, addedEnd-addedStart)
			for offset := 0; offset < count; offset++ {
				current := base
				if removedStart+offset < index {
					current.leftLine = removedStart + offset
					old := lines[current.leftLine]
					current.left = highlightedLine(highlighted.Old, old.OldNumber, old.Text)
				}
				if addedStart+offset < addedEnd {
					current.rightLine = addedStart + offset
					added := lines[current.rightLine]
					current.right = highlightedLine(highlighted.New, added.NewNumber, added.Text)
				}
				v.rows = append(v.rows, current)
			}
			index = addedEnd
		}
	}
}

func (v *diffView) appendFileHeader(fileIndex int, file *patch.File) {
	v.rows = append(v.rows, newEntry(fileRow, fileIndex, -1, -1, -1, file.DisplayPath, "", ""))
	for _, metadata := range file.Metadata {
		v.rows = append(v.rows, newEntry(metadataRow, fileIndex, -1, -1, -1, metadata, "", ""))
	}
}
