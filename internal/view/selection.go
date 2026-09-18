package view

import "github.com/eskelinenantti/review-my-slop/internal/patch"

func (v *diffView) BeginSelection(cursor Cursor) Selection {
	return Selection{First: cursor, Last: cursor}
}

func isSelected(selection *Selection, cursor Cursor) bool {
	if selection == nil {
		return false
	}
	first, last := selection.First.Coordinate.Y, selection.Last.Coordinate.Y
	if first == last && selection.First.Pane != selection.Last.Pane {
		return cursor.Coordinate.Y == first && (cursor.Pane == selection.First.Pane || cursor.Pane == selection.Last.Pane)
	}
	if selection.First.Pane != cursor.Pane {
		return false
	}
	if first > last {
		first, last = last, first
	}
	return cursor.Coordinate.Y >= first && cursor.Coordinate.Y <= last
}

func (v *diffView) ExtendSelection(selection Selection, cursor Cursor) (Selection, bool) {
	if !v.valid(selection.First) || !v.valid(cursor) {
		return selection, false
	}
	first := v.rows[selection.First.Coordinate.Y]
	last := v.rows[cursor.Coordinate.Y]
	if first.fileIndex != last.fileIndex || first.hunkIndex != last.hunkIndex {
		return selection, false
	}
	selection.Last = cursor
	return selection, true
}

func (v *diffView) Lines(selection Selection) []patch.Line {
	lines := make([]patch.Line, 0)
	for _, cursor := range v.selectionCursors(selection) {
		line, ok := v.Line(cursor)
		if !ok || (len(lines) > 0 && lines[len(lines)-1] == line) {
			continue
		}
		lines = append(lines, line)
	}
	return lines
}

func (v *diffView) selectionCursors(selection Selection) []Cursor {
	if _, ok := v.ExtendSelection(selection, selection.Last); !ok {
		return nil
	}
	first, last := selection.First.Coordinate.Y, selection.Last.Coordinate.Y
	if first > last {
		first, last = last, first
	}
	if first == last && selection.First.Pane != selection.Last.Pane {
		return []Cursor{selection.First, selection.Last}
	}
	cursors := make([]Cursor, 0, last-first+1)
	for y := first; y <= last; y++ {
		pane := selection.First.Pane
		if y == selection.Last.Coordinate.Y {
			pane = selection.Last.Pane
		}
		cursor := Cursor{Coordinate: Coordinate{Y: y}, Pane: pane}
		if v.valid(cursor) {
			cursors = append(cursors, cursor)
		}
	}
	return cursors
}

func (v *diffView) File(cursor Cursor) (patch.File, bool) {
	current, ok := v.rowAt(cursor)
	if !ok {
		return patch.File{}, false
	}
	return v.patch.Files[current.fileIndex], true
}

func (v *diffView) Hunk(cursor Cursor) (patch.Hunk, bool) {
	current, ok := v.rowAt(cursor)
	if !ok {
		return patch.Hunk{}, false
	}
	return v.patch.Files[current.fileIndex].Hunks[current.hunkIndex], true
}

func (v *diffView) Line(cursor Cursor) (patch.Line, bool) {
	current, ok := v.rowAt(cursor)
	if !ok {
		return patch.Line{}, false
	}
	return v.patch.Files[current.fileIndex].Hunks[current.hunkIndex].Lines[v.lineIndex(current, cursor.Pane)], true
}

func (v *diffView) FindCursor(file patch.File, hunk patch.Hunk, line patch.Line, nearby Coordinate, pane Pane) (Cursor, bool) {
	var sameText, sameKind, anyLine []Cursor
	for y, row := range v.rows {
		if !v.rowBelongsToHunk(row, file, hunk) {
			continue
		}
		for _, candidatePane := range []Pane{pane, pane.Other()} {
			candidate, ok := v.cursorAt(y, candidatePane)
			if !ok {
				continue
			}
			candidateLine, ok := v.Line(candidate)
			if !ok {
				continue
			}
			if sameLinePosition(candidateLine, line) {
				return candidate, true
			}
			if candidateLine.Kind == line.Kind && candidateLine.Text == line.Text {
				sameText = append(sameText, candidate)
			}
			if candidateLine.Kind == line.Kind {
				sameKind = append(sameKind, candidate)
			}
			anyLine = append(anyLine, candidate)
		}
	}
	for _, matches := range [][]Cursor{sameText, sameKind, anyLine} {
		if len(matches) > 0 {
			return closest(matches, nearby), true
		}
	}
	return Cursor{}, false
}

func (v *diffView) rowBelongsToHunk(current row, file patch.File, hunk patch.Hunk) bool {
	if current.fileIndex < 0 || current.fileIndex >= len(v.patch.Files) || !sameFile(v.patch.Files[current.fileIndex], file) {
		return false
	}
	if current.hunkIndex < 0 || current.hunkIndex >= len(v.patch.Files[current.fileIndex].Hunks) {
		return false
	}
	return v.patch.Files[current.fileIndex].Hunks[current.hunkIndex].Header == hunk.Header
}

func (v *diffView) rowAt(cursor Cursor) (row, bool) {
	if cursor.Coordinate.Y < 0 || cursor.Coordinate.Y >= len(v.rows) {
		return row{}, false
	}
	current := v.rows[cursor.Coordinate.Y]
	if v.lineIndex(current, cursor.Pane) < 0 {
		return row{}, false
	}
	return current, true
}

func sameLinePosition(first, second patch.Line) bool {
	return first.Kind == second.Kind && first.OldNumber == second.OldNumber && first.NewNumber == second.NewNumber
}

func sameFile(candidate, target patch.File) bool {
	if candidate.OldPath != "" && candidate.OldPath == target.OldPath {
		return true
	}
	return candidate.NewPath != "" && candidate.NewPath == target.NewPath
}

func sameFilePath(first, second patch.File) bool {
	return first.OldPath == second.OldPath && first.NewPath == second.NewPath
}

func closest(candidates []Cursor, nearby Coordinate) Cursor {
	best := candidates[0]
	for _, candidate := range candidates[1:] {
		if abs(candidate.Coordinate.Y-nearby.Y) < abs(best.Coordinate.Y-nearby.Y) {
			best = candidate
		}
	}
	return best
}

func abs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func highlightedSourceLine(lines []string, number patch.LineNumber, fallback string) string {
	if number <= 0 || int(number) > len(lines) {
		return fallback
	}
	return lines[int(number)-1]
}
