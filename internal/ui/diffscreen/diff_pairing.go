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
			next, ok := yieldLineBlock(lines, index, yield)
			if !ok {
				return
			}
			index = next
		}
	}
}

func yieldLineBlock(
	lines []patch.Line,
	start int,
	yield func(int, int) bool,
) (int, bool) {
	kind := lines[start].Kind
	if kind != patch.Addition && kind != patch.Deletion {
		return start + 1, yield(start, start)
	}
	addedStart := changeBlockEnd(lines, start, patch.Deletion)
	end := changeBlockEnd(lines, addedStart, patch.Addition)
	count := max(addedStart-start, end-addedStart)
	for offset := 0; offset < count; offset++ {
		left := blockLineIndex(start, addedStart, offset)
		right := blockLineIndex(addedStart, end, offset)
		if !yield(left, right) {
			return end, false
		}
	}
	return end, true
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
