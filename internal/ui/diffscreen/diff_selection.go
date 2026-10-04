package diffscreen

import (
	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

func (v *projection) extendSelection(selection diffSelection, cursor diffCursor) (diffSelection, bool) {
	if !v.hasSourceLine(selection.First) || !v.hasSourceLine(cursor) {
		return selection, false
	}
	first := v.rows[selection.First.row]
	last := v.rows[cursor.row]
	if first.file != last.file || first.hunk != last.hunk {
		return selection, false
	}
	selection.Last = cursor
	return selection, true
}

func (v *projection) selectedLines(selection diffSelection) []patch.Line {
	if _, ok := v.extendSelection(selection, selection.Last); !ok {
		return nil
	}
	first, last := selection.First.row, selection.Last.row
	if first > last {
		first, last = last, first
	}
	lines := make([]patch.Line, 0, last-first+1)
	if first == last && selection.First.pane != selection.Last.pane {
		current := v.rows[first]
		indices := []int{v.lineIndex(current, selection.First.pane), v.lineIndex(current, selection.Last.pane)}
		for _, index := range indices {
			if index >= 0 && (len(lines) == 0 || lines[len(lines)-1] != v.patch.Files[current.file].Hunks[current.hunk].Lines[index]) {
				lines = append(lines, v.patch.Files[current.file].Hunks[current.hunk].Lines[index])
			}
		}
		return lines
	}
	for y := first; y <= last; y++ {
		pane := selection.First.pane
		if y == selection.Last.row {
			pane = selection.Last.pane
		}
		if source := v.sourceAt(diffCursor{row: y, pane: pane}); source.valid {
			lines = append(lines, source.line)
		}
	}
	return lines
}

func (selection *diffSelection) contains(cursor diffCursor) bool {
	if selection == nil {
		return false
	}
	first, last := selection.First.row, selection.Last.row
	if first == last && selection.First.pane != selection.Last.pane {
		return cursor.row == first && (cursor.pane == selection.First.pane || cursor.pane == selection.Last.pane)
	}
	if selection.First.pane != cursor.pane {
		return false
	}
	if first > last {
		first, last = last, first
	}
	return cursor.row >= first && cursor.row <= last
}
