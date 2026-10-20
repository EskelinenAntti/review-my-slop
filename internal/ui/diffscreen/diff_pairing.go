package diffscreen

import (
	"iter"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

func unifiedLinePairs(lines []patch.Line) iter.Seq2[int, int] {
	return func(yield func(int, int) bool) {
		for index := range lines {
			if !yield(index, index) {
				return
			}
		}
	}
}

// splitLinePairs pairs adjacent deletion and addition blocks by source index.
// Context appears in both panes; -1 denotes an empty pane.
func splitLinePairs(lines []patch.Line) iter.Seq2[int, int] {
	return func(yield func(int, int) bool) {
		for index := 0; index < len(lines); {
			block := lineBlockAt(lines, index)
			if !block.yield(yield) {
				return
			}
			index = max(block.leftEnd, block.rightEnd)
		}
	}
}

// lineBlock describes half-open source ranges paired in the two panes.
type lineBlock struct {
	leftStart, leftEnd   int
	rightStart, rightEnd int
}

func lineBlockAt(lines []patch.Line, start int) lineBlock {
	kind := lines[start].Kind
	if kind != patch.Addition && kind != patch.Deletion {
		return lineBlock{start, start + 1, start, start + 1}
	}
	addedStart := changeBlockEnd(lines, start, patch.Deletion)
	end := changeBlockEnd(lines, addedStart, patch.Addition)
	return lineBlock{start, addedStart, addedStart, end}
}

func (b lineBlock) yield(yield func(int, int) bool) bool {
	count := max(b.leftEnd-b.leftStart, b.rightEnd-b.rightStart)
	for offset := 0; offset < count; offset++ {
		left := blockLineIndex(b.leftStart, b.leftEnd, offset)
		right := blockLineIndex(b.rightStart, b.rightEnd, offset)
		if !yield(left, right) {
			return false
		}
	}
	return true
}

func blockLineIndex(start, end, offset int) int {
	if start+offset < end {
		return start + offset
	}
	return -1
}

func changeBlockEnd(lines []patch.Line, start int, kind patch.LineKind) int {
	for start < len(lines) && lines[start].Kind == kind {
		start++
	}
	return start
}
