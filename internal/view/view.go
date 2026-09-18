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

const (
	unifiedGutterWidth = 14
	splitDividerWidth  = 3
	paneGutterWidth    = 6
	splitExtraWidth    = 2
)

type row struct {
	kind       rowKind
	fileIndex  int
	hunkIndex  int
	leftIndex  int
	rightIndex int
	text       string
	leftText   string
	rightText  string
}

type diffView struct {
	patch          patch.Patch
	rows           []row
	split          bool
	darkBackground bool
}

func NewUnifiedView(p patch.Patch, darkBackground bool) View {
	v := &diffView{patch: p, darkBackground: darkBackground}
	v.buildUnifiedRows()
	return v
}

func NewSideBySideView(p patch.Patch, darkBackground bool) View {
	v := &diffView{patch: p, split: true, darkBackground: darkBackground}
	v.buildSplitRows()
	return v
}

func (v *diffView) buildUnifiedRows() {
	for fileIndex := range v.patch.Files {
		v.appendFileRows(fileIndex, func(hunkIndex int, hunk *patch.Hunk, highlighted highlight.FileSources) {
			for lineIndex, line := range hunk.Lines {
				text := line.Text
				if line.Kind == patch.Deletion {
					text = highlightedSourceLine(highlighted.Old, line.OldNumber, text)
				} else {
					text = highlightedSourceLine(highlighted.New, line.NewNumber, text)
				}
				v.rows = append(v.rows, row{kind: lineRow, fileIndex: fileIndex, hunkIndex: hunkIndex, leftIndex: lineIndex, rightIndex: lineIndex, text: text})
			}
		})
	}
}

func (v *diffView) buildSplitRows() {
	for fileIndex := range v.patch.Files {
		v.appendFileRows(fileIndex, func(hunkIndex int, hunk *patch.Hunk, highlighted highlight.FileSources) {
			v.appendSplitHunk(fileIndex, hunkIndex, hunk, highlighted)
		})
	}
}

func (v *diffView) appendSplitHunk(fileIndex, hunkIndex int, hunk *patch.Hunk, highlighted highlight.FileSources) {
	for index := 0; index < len(hunk.Lines); {
		switch hunk.Lines[index].Kind {
		case patch.Context:
			v.appendSplitContext(fileIndex, hunkIndex, index, hunk.Lines[index], highlighted)
			index++
		case patch.Addition:
			v.appendSplitAddition(fileIndex, hunkIndex, index, hunk.Lines[index], highlighted)
			index++
		case patch.Deletion:
			index = v.appendSplitChangeBlock(fileIndex, hunkIndex, index, hunk, highlighted)
		default:
			index++
		}
	}
}

func (v *diffView) appendSplitContext(fileIndex, hunkIndex, lineIndex int, line patch.Line, highlighted highlight.FileSources) {
	text := highlightedSourceLine(highlighted.New, line.NewNumber, line.Text)
	v.rows = append(v.rows, row{
		kind: lineRow, fileIndex: fileIndex, hunkIndex: hunkIndex,
		leftIndex: lineIndex, rightIndex: lineIndex, leftText: text, rightText: text,
	})
}

func (v *diffView) appendSplitAddition(fileIndex, hunkIndex, lineIndex int, line patch.Line, highlighted highlight.FileSources) {
	text := highlightedSourceLine(highlighted.New, line.NewNumber, line.Text)
	v.rows = append(v.rows, row{
		kind: lineRow, fileIndex: fileIndex, hunkIndex: hunkIndex,
		leftIndex: -1, rightIndex: lineIndex, rightText: text,
	})
}

func (v *diffView) appendSplitChangeBlock(fileIndex, hunkIndex, start int, hunk *patch.Hunk, highlighted highlight.FileSources) int {
	removedStart := start
	removedEnd := start
	for removedEnd < len(hunk.Lines) && hunk.Lines[removedEnd].Kind == patch.Deletion {
		removedEnd++
	}
	addedStart, addedEnd := removedEnd, removedEnd
	for addedEnd < len(hunk.Lines) && hunk.Lines[addedEnd].Kind == patch.Addition {
		addedEnd++
	}
	for offset := 0; offset < max(removedEnd-removedStart, addedEnd-addedStart); offset++ {
		current := row{kind: lineRow, fileIndex: fileIndex, hunkIndex: hunkIndex, leftIndex: -1, rightIndex: -1}
		if removedStart+offset < removedEnd {
			current.leftIndex = removedStart + offset
			old := hunk.Lines[current.leftIndex]
			current.leftText = highlightedSourceLine(highlighted.Old, old.OldNumber, old.Text)
		}
		if addedStart+offset < addedEnd {
			current.rightIndex = addedStart + offset
			added := hunk.Lines[current.rightIndex]
			current.rightText = highlightedSourceLine(highlighted.New, added.NewNumber, added.Text)
		}
		v.rows = append(v.rows, current)
	}
	return addedEnd
}

func (v *diffView) appendFileRows(fileIndex int, appendHunk func(int, *patch.Hunk, highlight.FileSources)) {
	file := &v.patch.Files[fileIndex]
	v.rows = append(v.rows, row{kind: fileRow, fileIndex: fileIndex, hunkIndex: -1, leftIndex: -1, rightIndex: -1, text: file.DisplayPath})
	for _, metadata := range file.Metadata {
		v.rows = append(v.rows, row{kind: metadataRow, fileIndex: fileIndex, hunkIndex: -1, leftIndex: -1, rightIndex: -1, text: metadata})
	}
	highlighted := v.highlight(file)
	for hunkIndex := range file.Hunks {
		hunk := &file.Hunks[hunkIndex]
		v.rows = append(v.rows, row{kind: hunkRow, fileIndex: fileIndex, hunkIndex: hunkIndex, leftIndex: -1, rightIndex: -1, text: hunkHeader(hunk.Header)})
		appendHunk(hunkIndex, hunk, highlighted)
	}
}

func (v *diffView) highlight(file *patch.File) highlight.FileSources {
	return highlight.Highlight(file.Path(), file.OldSource, file.NewSource, v.darkBackground)
}

func hunkHeader(header string) string {
	if strings.HasPrefix(header, "@@") {
		return header
	}
	return "@@ " + header
}
