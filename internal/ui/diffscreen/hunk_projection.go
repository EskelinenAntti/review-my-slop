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

			removed := takeLineRun(lines, index, patch.Deletion)
			added := takeLineRun(lines, removed.end, patch.Addition)
			rows := max(removed.length(), added.length())
			for offset := 0; offset < rows; offset++ {
				if !yield(removed.lineAt(offset), added.lineAt(offset)) {
					return
				}
			}
			index = added.end
		}
	}
}

type hunkLineRun struct {
	start, end int
}

func takeLineRun(
	lines []patch.Line, start int, kind patch.LineKind,
) hunkLineRun {
	end := start
	for end < len(lines) && lines[end].Kind == kind {
		end++
	}
	return hunkLineRun{start: start, end: end}
}

func (r hunkLineRun) length() int {
	return r.end - r.start
}

// lineAt returns -1 when this run has no line at the paired row's offset.
func (r hunkLineRun) lineAt(offset int) int {
	if offset >= r.length() {
		return -1
	}
	return r.start + offset
}
