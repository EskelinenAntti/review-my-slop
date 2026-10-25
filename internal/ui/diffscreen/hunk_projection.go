package diffscreen

import (
	"iter"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

func unifiedHunkLinePairs(lines []patch.Line) iter.Seq2[int, int] {
	return func(yield func(int, int) bool) {
		for index := range lines {
			if !yield(index, index) {
				return
			}
		}
	}
}

// splitHunkLinePairs yields source line indices for each display row; -1 marks
// an empty pane. Deletions pair only with immediately following additions.
func splitHunkLinePairs(lines []patch.Line) iter.Seq2[int, int] {
	return func(yield func(int, int) bool) {
		for index := 0; index < len(lines); {
			if lines[index].Kind != patch.Deletion {
				leftLine := index
				if lines[index].Kind == patch.Addition {
					leftLine = -1
				}
				if !yield(leftLine, index) {
					return
				}
				index++
				continue
			}

			removedStart := index
			for index < len(lines) && lines[index].Kind == patch.Deletion {
				index++
			}
			addedStart := index
			for index < len(lines) && lines[index].Kind == patch.Addition {
				index++
			}
			count := max(addedStart-removedStart, index-addedStart)
			for offset := 0; offset < count; offset++ {
				leftLine, rightLine := -1, -1
				if removedStart+offset < addedStart {
					leftLine = removedStart + offset
				}
				if addedStart+offset < index {
					rightLine = addedStart + offset
				}
				if !yield(leftLine, rightLine) {
					return
				}
			}
		}
	}
}
