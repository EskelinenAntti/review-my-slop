package ui

import "github.com/eskelinenantti/review-my-slop/internal/patch"

func (v *diffView) BeginSelection(cursor Cursor) Selection {
	return Selection{First: cursor, Last: cursor}
}

func (v *diffView) ExtendSelection(selection Selection, cursor Cursor) (Selection, bool) {
	firstCursor := selection.First
	valid, rows := v.valid, v.rows
	if !valid(firstCursor) || !valid(cursor) {
		return selection, false
	}
	first := rows[firstCursor.Coordinate.Y]
	last := rows[cursor.Coordinate.Y]
	if first.file != last.file || first.hunk != last.hunk {
		return selection, false
	}
	selection.Last = cursor
	return selection, true
}

func (v *diffView) Lines(selection Selection) []diffLine {
	return v.selectedLines(selection, true)
}

func (v *diffView) selectedLines(selection Selection, collapseDuplicates bool) []diffLine {
	firstCursor, lastCursor := selection.First, selection.Last
	firstPane, lastPane := firstCursor.Pane, lastCursor.Pane
	if _, ok := v.ExtendSelection(selection, lastCursor); !ok {
		return nil
	}
	first, last := firstCursor.Coordinate.Y, lastCursor.Coordinate.Y
	if first > last {
		first, last = last, first
	}
	lines := make([]diffLine, 0, last-first+1)
	if first == last && firstPane != lastPane {
		current := v.rows[first]
		for _, pane := range []Pane{firstPane, lastPane} {
			index := v.lineIndex(current, pane)
			if index >= 0 {
				line := v.patch.Files[current.file].Hunks[current.hunk].Lines[index]
				if !collapseDuplicates || len(lines) == 0 || lines[0] != line {
					lines = append(lines, line)
				}
			}
		}
		return lines
	}
	for y := first; y <= last; y++ {
		pane := firstPane
		if y == last {
			pane = lastPane
		}
		if line, ok := v.Line(lineCursor(y, pane)); ok {
			lines = append(lines, line)
		}
	}
	return lines
}

func (v *diffView) Anchor(selection Selection) (commentAnchor, error) {
	lines := v.selectedLines(selection, false)
	if len(lines) == 0 {
		return commentAnchor{}, formatError("select code lines before commenting")
	}
	first := v.rows[selection.First.Coordinate.Y]
	file := v.patch.Files[first.file]
	anchor := commentAnchor{FilePath: file.Path()}
	var quotedLines []string
	for _, line := range lines {
		prefix := " "
		kind := line.Kind
		switch kind {
		case patch.Addition:
			prefix = "+"
		case patch.Deletion:
			prefix = "-"
		}
		quotedLines = append(quotedLines, prefix+line.Text)
		accumulateRange(&anchor.OldStart, &anchor.OldEnd, int(line.OldNumber))
		accumulateRange(&anchor.NewStart, &anchor.NewEnd, int(line.NewNumber))
	}
	anchor.QuotedLines = quotedLines
	return anchor, nil
}

func (v *diffView) File(cursor Cursor) (diffFile, bool) {
	current, ok := v.rowAt(cursor)
	if !ok {
		return diffFile{}, false
	}
	return v.patch.Files[current.file], true
}

func (v *diffView) Hunk(cursor Cursor) (diffHunk, bool) {
	current, ok := v.rowAt(cursor)
	if !ok {
		return diffHunk{}, false
	}
	return v.patch.Files[current.file].Hunks[current.hunk], true
}

func (v *diffView) Line(cursor Cursor) (diffLine, bool) {
	current, ok := v.rowAt(cursor)
	if !ok {
		return diffLine{}, false
	}
	index := v.lineIndex(current, cursor.Pane)
	return v.patch.Files[current.file].Hunks[current.hunk].Lines[index], true
}

func (v *diffView) rowAt(cursor Cursor) (entry, bool) {
	if !v.valid(cursor) {
		return entry{}, false
	}
	return v.rows[cursor.Coordinate.Y], true
}

func (v *diffView) FindCursor(file diffFile, hunk diffHunk, line diffLine, nearby Coordinate, pane Pane) (Cursor, bool) {
	var textMatch, kindMatch, nearbyMatch Cursor
	var textDistance, kindDistance, nearbyDistance int
	var hasText, hasKind, hasNearby bool
	files := v.patch.Files
	for y, current := range v.rows {
		fileIndex, hunkIndex := current.file, current.hunk
		if fileIndex < 0 || !sameFile(files[fileIndex], file) || hunkIndex < 0 || files[fileIndex].Hunks[hunkIndex].Header != hunk.Header {
			continue
		}
		for _, candidatePane := range []Pane{pane, pane.Other()} {
			candidate, ok := v.cursorAt(y, candidatePane)
			if !ok {
				continue
			}
			candidateLine, _ := v.Line(candidate)
			candidateKind, kind := candidateLine.Kind, line.Kind
			if candidateKind == kind && candidateLine.OldNumber == line.OldNumber && candidateLine.NewNumber == line.NewNumber {
				return candidate, true
			}
			distance := abs(candidate.Coordinate.Y - nearby.Y)
			if !hasNearby || distance < nearbyDistance {
				nearbyMatch, nearbyDistance, hasNearby = candidate, distance, true
			}
			if candidateKind == kind {
				if !hasKind || distance < kindDistance {
					kindMatch, kindDistance, hasKind = candidate, distance, true
				}
				if candidateLine.Text == line.Text && (!hasText || distance < textDistance) {
					textMatch, textDistance, hasText = candidate, distance, true
				}
			}
		}
	}
	if hasText {
		return textMatch, true
	}
	if hasKind {
		return kindMatch, true
	}
	if hasNearby {
		return nearbyMatch, true
	}
	return Cursor{}, false
}

func sameFile(candidate, target diffFile) bool {
	oldPath, newPath := candidate.OldPath, candidate.NewPath
	return oldPath != "" && oldPath == target.OldPath ||
		newPath != "" && newPath == target.NewPath
}

func accumulateRange(start, end *int, value int) {
	if value == 0 {
		return
	}
	if *start == 0 || value < *start {
		*start = value
	}
	if value > *end {
		*end = value
	}
}

func abs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func highlightedLine(lines []string, number diffLineNumber, fallback string) string {
	if number <= 0 || int(number) > len(lines) {
		return fallback
	}
	return lines[int(number)-1]
}
