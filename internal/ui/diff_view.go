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
	patch diffPatch
	rows  []entry
	split bool
	dark  bool
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
	files := v.patch.Files
	for fileIndex := range files {
		file := &files[fileIndex]
		hunks := file.Hunks
		v.appendFileHeader(fileIndex, file)
		highlighted := v.highlight(file)
		for hunkIndex := range hunks {
			hunk := &hunks[hunkIndex]
			v.appendHunkHeader(fileIndex, hunkIndex, hunk)
			if v.split {
				v.buildSplitHunk(fileIndex, hunkIndex, hunk, highlighted)
			} else {
				v.buildUnifiedHunk(fileIndex, hunkIndex, hunk, highlighted)
			}
		}
	}
}

func (v *diffView) buildUnifiedHunk(file, index int, hunk *diffHunk, highlighted Pair) {
	for lineIndex, line := range hunk.Lines {
		text := line.Text
		if line.Kind == patch.Deletion {
			text = highlightedLine(highlighted.Old, line.OldNumber, text)
		} else {
			text = highlightedLine(highlighted.New, line.NewNumber, text)
		}
		v.rows = append(v.rows, entry{kind: lineRow, file: file, hunk: index, leftLine: lineIndex, rightLine: lineIndex, text: text})
	}
}

func (v *diffView) buildSplitHunk(fileIndex, hunkIndex int, hunk *diffHunk, highlighted Pair) {
	lines := hunk.Lines
	addition, deletion := patch.Addition, patch.Deletion
	appendLine := v.appendSplitLine
	for index := 0; index < len(lines); {
		line := lines[index]
		switch line.Kind {
		case patch.Context:
			text := highlightedLine(highlighted.New, line.NewNumber, line.Text)
			v.rows = append(v.rows, entry{kind: lineRow, file: fileIndex, hunk: hunkIndex, leftLine: index, rightLine: index, left: text, right: text})
			index++
		case addition:
			appendLine(fileIndex, hunkIndex, hunk, highlighted, -1, index)
			index++
		case deletion:
			removedStart := index
			for index < len(lines) && lines[index].Kind == deletion {
				index++
			}
			addedStart, addedEnd := index, index
			for addedEnd < len(lines) && lines[addedEnd].Kind == addition {
				addedEnd++
			}
			count := max(index-removedStart, addedEnd-addedStart)
			for offset := 0; offset < count; offset++ {
				left, right := -1, -1
				if removedStart+offset < index {
					left = removedStart + offset
				}
				if addedStart+offset < addedEnd {
					right = addedStart + offset
				}
				appendLine(fileIndex, hunkIndex, hunk, highlighted, left, right)
			}
			index = addedEnd
		}
	}
}

func (v *diffView) appendSplitLine(fileIndex, hunkIndex int, hunk *diffHunk, highlighted Pair, left, right int) {
	current := entry{kind: lineRow, file: fileIndex, hunk: hunkIndex, leftLine: left, rightLine: right}
	lines := hunk.Lines
	if left >= 0 {
		line := lines[left]
		current.left = highlightedLine(highlighted.Old, line.OldNumber, line.Text)
	}
	if right >= 0 {
		line := lines[right]
		current.right = highlightedLine(highlighted.New, line.NewNumber, line.Text)
	}
	v.rows = append(v.rows, current)
}

func (v *diffView) appendFileHeader(index int, file *diffFile) {
	rows := v.rows
	rows = append(rows, entry{kind: fileRow, file: index, hunk: -1, leftLine: -1, rightLine: -1, text: file.DisplayPath})
	for _, metadata := range file.Metadata {
		rows = append(rows, entry{kind: metadataRow, file: index, hunk: -1, leftLine: -1, rightLine: -1, text: metadata})
	}
	v.rows = rows
}

func (v *diffView) appendHunkHeader(file, index int, hunk *diffHunk) {
	v.rows = append(v.rows, entry{kind: hunkRow, file: file, hunk: index, leftLine: -1, rightLine: -1, text: hunkHeader(hunk.Header)})
}

func (v *diffView) highlight(file *diffFile) Pair {
	return Sources(file.Path(), file.OldSource, file.NewSource, v.dark)
}

func hunkHeader(header string) string {
	if strings.HasPrefix(header, "@@") {
		return header
	}
	return "@@ " + header
}
