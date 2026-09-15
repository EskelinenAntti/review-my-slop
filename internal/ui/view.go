package ui

import (
	"strings"

	"github.com/eskelinenantti/review-my-slop/internal/diff"
)

func NewUnifiedView(changes diff.ChangeSet, dark bool) View {
	view := &diffView{changes: changes, dark: dark}
	view.buildUnified()
	return view
}

func NewSideBySideView(changes diff.ChangeSet, dark bool) View {
	view := &diffView{changes: changes, split: true, dark: dark}
	view.buildSplit()
	return view
}

func (v *diffView) buildUnified() {
	for fileIndex := range v.changes.Files {
		file := &v.changes.Files[fileIndex]
		v.rows = append(v.rows, row{kind: fileRow, file: fileIndex, hunk: -1, leftLine: -1, rightLine: -1, text: file.DisplayPath})
		for _, metadata := range file.Metadata {
			v.rows = append(v.rows, row{kind: metadataRow, file: fileIndex, hunk: -1, leftLine: -1, rightLine: -1, text: metadata})
		}
		highlighted := v.highlight(file)
		for hunkIndex := range file.Hunks {
			hunk := &file.Hunks[hunkIndex]
			v.rows = append(v.rows, row{kind: hunkRow, file: fileIndex, hunk: hunkIndex, leftLine: -1, rightLine: -1, text: hunkHeader(hunk.Header)})
			for lineIndex, line := range hunk.Lines {
				text := line.Text
				if line.Kind == diff.Deletion {
					text = highlightedLine(highlighted.Old, line.OldNumber, text)
				} else {
					text = highlightedLine(highlighted.New, line.NewNumber, text)
				}
				v.rows = append(v.rows, row{kind: lineRow, file: fileIndex, hunk: hunkIndex, leftLine: lineIndex, rightLine: lineIndex, text: text})
			}
		}
	}
}

func (v *diffView) buildSplit() {
	for fileIndex := range v.changes.Files {
		file := &v.changes.Files[fileIndex]
		v.rows = append(v.rows, row{kind: fileRow, file: fileIndex, hunk: -1, leftLine: -1, rightLine: -1, text: file.DisplayPath})
		for _, metadata := range file.Metadata {
			v.rows = append(v.rows, row{kind: metadataRow, file: fileIndex, hunk: -1, leftLine: -1, rightLine: -1, text: metadata})
		}
		highlighted := v.highlight(file)
		for hunkIndex := range file.Hunks {
			hunk := &file.Hunks[hunkIndex]
			v.rows = append(v.rows, row{kind: hunkRow, file: fileIndex, hunk: hunkIndex, leftLine: -1, rightLine: -1, text: hunkHeader(hunk.Header)})
			for index := 0; index < len(hunk.Lines); {
				line := hunk.Lines[index]
				switch line.Kind {
				case diff.Context:
					text := highlightedLine(highlighted.New, line.NewNumber, line.Text)
					v.rows = append(v.rows, row{kind: lineRow, file: fileIndex, hunk: hunkIndex, leftLine: index, rightLine: index, left: text, right: text})
					index++
				case diff.Addition:
					text := highlightedLine(highlighted.New, line.NewNumber, line.Text)
					v.rows = append(v.rows, row{kind: lineRow, file: fileIndex, hunk: hunkIndex, leftLine: -1, rightLine: index, right: text})
					index++
				case diff.Deletion:
					removedStart := index
					for index < len(hunk.Lines) && hunk.Lines[index].Kind == diff.Deletion {
						index++
					}
					addedStart, addedEnd := index, index
					for addedEnd < len(hunk.Lines) && hunk.Lines[addedEnd].Kind == diff.Addition {
						addedEnd++
					}
					count := max(index-removedStart, addedEnd-addedStart)
					for offset := 0; offset < count; offset++ {
						current := row{kind: lineRow, file: fileIndex, hunk: hunkIndex, leftLine: -1, rightLine: -1}
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

func (v *diffView) highlight(file *diff.File) highlightedSources {
	return highlightSources(file.Path(), file.OldSource, file.NewSource, v.dark)
}

func hunkHeader(header string) string {
	if strings.HasPrefix(header, "@@") {
		return header
	}
	return "@@ " + header
}
