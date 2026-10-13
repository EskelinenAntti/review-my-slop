package diffscreen

import "github.com/eskelinenantti/review-my-slop/internal/patch"

// projectHunkRows maps source lines to display rows. Split mode pairs each
// deletion block with the following additions; unmatched panes use -1.
func (v *patchProjection) projectHunkRows(lines []patch.Line) [][2]int {
	rows := make([][2]int, 0, len(lines))
	for index := 0; index < len(lines); {
		if !v.split || lines[index].Kind == patch.Context {
			rows = append(rows, [2]int{index, index})
			index++
			continue
		}
		if lines[index].Kind == patch.Addition {
			rows = append(rows, [2]int{-1, index})
			index++
			continue
		}
		addedStart := changeBlockEnd(lines, index, patch.Deletion)
		end := changeBlockEnd(lines, addedStart, patch.Addition)
		rows = append(rows, pairAdjacentChangeBlocks(index, addedStart, end)...)
		index = end
	}
	return rows
}

func changeBlockEnd(lines []patch.Line, start int, kind patch.LineKind) int {
	for start < len(lines) && lines[start].Kind == kind {
		start++
	}
	return start
}

func pairAdjacentChangeBlocks(removedStart, addedStart, end int) [][2]int {
	count := max(addedStart-removedStart, end-addedStart)
	rows := make([][2]int, 0, count)
	for offset := 0; offset < count; offset++ {
		indices := [2]int{-1, -1}
		if removedStart+offset < addedStart {
			indices[0] = removedStart + offset
		}
		if addedStart+offset < end {
			indices[1] = addedStart + offset
		}
		rows = append(rows, indices)
	}
	return rows
}
