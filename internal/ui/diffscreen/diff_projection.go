package diffscreen

import (
	"iter"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

// unifiedLinePairs retains source order with the same index in both panes.
func unifiedLinePairs(lines []patch.Line) iter.Seq2[int, int] {
	return func(yield func(int, int) bool) {
		for index := range lines {
			if !yield(index, index) {
				return
			}
		}
	}
}

// splitLinePairs pairs adjacent deletion and addition blocks, yielding source
// indexes for each display row and using -1 for an empty pane.
func splitLinePairs(lines []patch.Line) iter.Seq2[int, int] {
	return func(yield func(int, int) bool) {
		for index := 0; index < len(lines); {
			kind := lines[index].Kind
			if kind != patch.Deletion && kind != patch.Addition {
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
				leftLine, rightLine := -1, -1
				if index+offset < addedStart {
					leftLine = index + offset
				}
				if addedStart+offset < end {
					rightLine = addedStart + offset
				}
				if !yield(leftLine, rightLine) {
					return
				}
			}
			index = end
		}
	}
}

func changeBlockEnd(lines []patch.Line, start int, kind patch.LineKind) int {
	for start < len(lines) && lines[start].Kind == kind {
		start++
	}
	return start
}
