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
			kind := lines[index].Kind
			if kind != patch.Addition && kind != patch.Deletion {
				if !yield(index, index) {
					return
				}
				index++
				continue
			}
			addedStart := changeBlockEnd(lines, index, patch.Deletion)
			end := changeBlockEnd(lines, addedStart, patch.Addition)
			count := max(addedStart-index, end-addedStart)
			for offset := 0; offset < count; offset++ {
				left := blockLineIndex(index, addedStart, offset)
				right := blockLineIndex(addedStart, end, offset)
				if !yield(left, right) {
					return
				}
			}
			index = end
		}
	}
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
