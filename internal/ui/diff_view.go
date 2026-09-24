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

func newDiffView(p patch.Patch, dark, split bool) *diffView {
	v := &diffView{patch: p, split: split, dark: dark}
	v.build()
	return v
}

func (v *diffView) build() {
	rows := v.rows
	for fileIndex, file := range v.patch.Files {
		rows = append(rows, entry{fileRow, fileIndex, -1, -1, -1, file.DisplayPath, "", ""})
		for _, metadata := range file.Metadata {
			rows = append(rows, entry{metadataRow, fileIndex, -1, -1, -1, metadata, "", ""})
		}
		filename := file.NewPath
		if filename == "" {
			filename = file.OldPath
		}
		highlighted := Pair{render(filename, file.OldSource, v.dark), render(filename, file.NewSource, v.dark)}
		for hunkIndex, hunk := range file.Hunks {
			header := hunk.Header
			if !strings.HasPrefix(header, "@@") {
				header = "@@ " + header
			}
			rows = append(rows, entry{hunkRow, fileIndex, hunkIndex, -1, -1, header, "", ""})
			if v.split {
				rows = appendSplitLines(rows, fileIndex, hunkIndex, hunk, highlighted)
				continue
			}
			for lineIndex, line := range hunk.Lines {
				text := line.Text
				if line.Kind == patch.Deletion {
					text = highlightedLine(highlighted.Old, line.OldNumber, text)
				} else {
					text = highlightedLine(highlighted.New, line.NewNumber, text)
				}
				rows = append(rows, entry{lineRow, fileIndex, hunkIndex, lineIndex, lineIndex, text, "", ""})
			}
		}
	}
	v.rows = rows
}

func appendSplitLines(rows []entry, fileIndex, hunkIndex int, hunk patch.Hunk, highlighted Pair) []entry {
	lines := hunk.Lines
	for index := 0; index < len(lines); {
		line := lines[index]
		if line.Kind != patch.Deletion {
			text := highlightedLine(highlighted.New, line.NewNumber, line.Text)
			left := -1
			if line.Kind == patch.Context {
				left = index
			}
			rows = append(rows, entry{lineRow, fileIndex, hunkIndex, left, index, "", text, text})
			index++
			continue
		}
		removedStart := index
		for index < len(lines) && lines[index].Kind == patch.Deletion {
			index++
		}
		addedStart, addedEnd := index, index
		for addedEnd < len(lines) && lines[addedEnd].Kind == patch.Addition {
			addedEnd++
		}
		count := max(index-removedStart, addedEnd-addedStart)
		for offset := range count {
			leftLine, rightLine := -1, -1
			left, right := "", ""
			if removedStart+offset < index {
				leftLine = removedStart + offset
				old := lines[leftLine]
				left = highlightedLine(highlighted.Old, old.OldNumber, old.Text)
			}
			if addedStart+offset < addedEnd {
				rightLine = addedStart + offset
				added := lines[rightLine]
				right = highlightedLine(highlighted.New, added.NewNumber, added.Text)
			}
			rows = append(rows, entry{lineRow, fileIndex, hunkIndex, leftLine, rightLine, "", left, right})
		}
		index = addedEnd
	}
	return rows
}
