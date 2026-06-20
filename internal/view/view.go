package view

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/eskelinenantti/review-my-slop/internal/highlight"
	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/review"
)

type rowKind uint8

const (
	fileRow rowKind = iota
	metadataRow
	hunkRow
	lineRow
)

type entry struct {
	kind              rowKind
	file, hunk        int
	leftLine          int
	rightLine         int
	text, left, right string
}

type diffView struct {
	patch patch.Patch
	rows  []entry
	split bool
	dark  bool
}

func NewUnifiedView(p patch.Patch, dark bool) View {
	v := &diffView{patch: p, dark: dark}
	v.buildUnified()
	return v
}

func NewSplitView(p patch.Patch, dark bool) View {
	v := &diffView{patch: p, split: true, dark: dark}
	v.buildSplit()
	return v
}

func (v *diffView) buildUnified() {
	for fileIndex := range v.patch.Files {
		file := &v.patch.Files[fileIndex]
		v.rows = append(v.rows, entry{kind: fileRow, file: fileIndex, hunk: -1, leftLine: -1, rightLine: -1, text: file.DisplayPath})
		for _, metadata := range file.Metadata {
			v.rows = append(v.rows, entry{kind: metadataRow, file: fileIndex, hunk: -1, leftLine: -1, rightLine: -1, text: metadata})
		}
		highlighted := highlight.Sources(file.Path(), file.OldSource, file.NewSource, v.dark)
		for hunkIndex := range file.Hunks {
			hunk := &file.Hunks[hunkIndex]
			v.rows = append(v.rows, entry{kind: hunkRow, file: fileIndex, hunk: hunkIndex, leftLine: -1, rightLine: -1, text: hunkHeader(hunk.Header)})
			for lineIndex, line := range hunk.Lines {
				text := line.Text
				if line.Kind == patch.Deletion {
					text = highlightedLine(highlighted.Old, line.OldNumber, text)
				} else {
					text = highlightedLine(highlighted.New, line.NewNumber, text)
				}
				v.rows = append(v.rows, entry{kind: lineRow, file: fileIndex, hunk: hunkIndex, leftLine: lineIndex, rightLine: lineIndex, text: text})
			}
		}
	}
}

func (v *diffView) buildSplit() {
	for fileIndex := range v.patch.Files {
		file := &v.patch.Files[fileIndex]
		v.rows = append(v.rows, entry{kind: fileRow, file: fileIndex, hunk: -1, leftLine: -1, rightLine: -1, text: file.DisplayPath})
		for _, metadata := range file.Metadata {
			v.rows = append(v.rows, entry{kind: metadataRow, file: fileIndex, hunk: -1, leftLine: -1, rightLine: -1, text: metadata})
		}
		highlighted := highlight.Sources(file.Path(), file.OldSource, file.NewSource, v.dark)
		for hunkIndex := range file.Hunks {
			hunk := &file.Hunks[hunkIndex]
			v.rows = append(v.rows, entry{kind: hunkRow, file: fileIndex, hunk: hunkIndex, leftLine: -1, rightLine: -1, text: hunkHeader(hunk.Header)})
			for index := 0; index < len(hunk.Lines); {
				line := hunk.Lines[index]
				switch line.Kind {
				case patch.Context:
					text := highlightedLine(highlighted.New, line.NewNumber, line.Text)
					v.rows = append(v.rows, entry{kind: lineRow, file: fileIndex, hunk: hunkIndex, leftLine: index, rightLine: index, left: text, right: text})
					index++
				case patch.Addition:
					text := highlightedLine(highlighted.New, line.NewNumber, line.Text)
					v.rows = append(v.rows, entry{kind: lineRow, file: fileIndex, hunk: hunkIndex, leftLine: -1, rightLine: index, right: text})
					index++
				case patch.Deletion:
					removedStart := index
					for index < len(hunk.Lines) && hunk.Lines[index].Kind == patch.Deletion {
						index++
					}
					addedStart, addedEnd := index, index
					for addedEnd < len(hunk.Lines) && hunk.Lines[addedEnd].Kind == patch.Addition {
						addedEnd++
					}
					count := max(index-removedStart, addedEnd-addedStart)
					for offset := 0; offset < count; offset++ {
						current := entry{kind: lineRow, file: fileIndex, hunk: hunkIndex, leftLine: -1, rightLine: -1}
						if removedStart+offset < index {
							current.leftLine = removedStart + offset
							old := hunk.Lines[current.leftLine]
							current.left = highlightedLine(highlighted.Old, old.OldNumber, old.Text)
						}
						if addedStart+offset < addedEnd {
							current.rightLine = addedStart + offset
							added := hunk.Lines[current.rightLine]
							current.right = highlightedLine(highlighted.New, added.NewNumber, added.Text)
						}
						v.rows = append(v.rows, current)
					}
					index = addedEnd
				}
			}
		}
	}
}

func hunkHeader(header string) string {
	if strings.HasPrefix(header, "@@") {
		return header
	}
	return "@@ " + header
}

func (v *diffView) valid(cursor Cursor) bool {
	return cursor.Coordinate.Y >= 0 && cursor.Coordinate.Y < len(v.rows) && v.lineIndex(v.rows[cursor.Coordinate.Y], cursor.Pane) >= 0
}

func (v *diffView) lineIndex(current entry, pane Pane) int {
	if current.kind != lineRow {
		return -1
	}
	if !v.split || pane == Right {
		return current.rightLine
	}
	return current.leftLine
}

func (v *diffView) cursorAt(y int, pane Pane) (Cursor, bool) {
	cursor := Cursor{Coordinate: Coordinate{Y: y}, Pane: pane}
	return cursor, v.valid(cursor)
}

func (v *diffView) First() (Cursor, bool) {
	if cursor, ok := v.scan(-1, Right, Forward, false); ok {
		return cursor, true
	}
	return v.scan(-1, Left, Forward, false)
}

func (v *diffView) Last() (Cursor, bool) {
	if cursor, ok := v.scan(len(v.rows), Right, Backward, false); ok {
		return cursor, true
	}
	return v.scan(len(v.rows), Left, Backward, false)
}

func (v *diffView) scan(start int, pane Pane, direction Direction, wrap bool) (Cursor, bool) {
	if len(v.rows) == 0 {
		return Cursor{}, false
	}
	y := start
	for count := 0; count < len(v.rows); count++ {
		y += int(direction)
		if y < 0 || y >= len(v.rows) {
			if !wrap {
				return Cursor{}, false
			}
			if y < 0 {
				y = len(v.rows) - 1
			} else {
				y = 0
			}
		}
		if cursor, ok := v.cursorAt(y, pane); ok {
			return cursor, true
		}
	}
	return Cursor{}, false
}

func (v *diffView) Move(cursor Cursor, direction Direction) (Cursor, bool) {
	if !v.valid(cursor) {
		return Cursor{}, false
	}
	return v.scan(cursor.Coordinate.Y, cursor.Pane, direction, false)
}

func (v *diffView) Search(query string, cursor Cursor, direction Direction) (Cursor, bool) {
	if query == "" || !v.valid(cursor) {
		return Cursor{}, false
	}
	query = strings.ToLower(query)
	y := cursor.Coordinate.Y
	for count := 0; count < len(v.rows)-1; count++ {
		y += int(direction)
		if y < 0 {
			y = len(v.rows) - 1
		}
		if y >= len(v.rows) {
			y = 0
		}
		current := v.rows[y]
		for _, pane := range []Pane{cursor.Pane, cursor.Pane.Other()} {
			candidate, ok := v.cursorAt(y, pane)
			if !ok || !v.split && pane != cursor.Pane {
				continue
			}
			line, _ := v.Line(candidate)
			if strings.Contains(strings.ToLower(line.Text), query) {
				return candidate, true
			}
		}
		if current.kind != lineRow && strings.Contains(strings.ToLower(ansi.Strip(current.text)), query) {
			if candidate, ok := v.cursorNearRow(y, cursor.Pane, direction); ok {
				return candidate, true
			}
		}
	}
	return Cursor{}, false
}

func (v *diffView) cursorNearRow(y int, pane Pane, direction Direction) (Cursor, bool) {
	for distance := 1; distance <= len(v.rows); distance++ {
		for _, candidateY := range []int{y + int(direction)*distance, y - int(direction)*distance} {
			if candidateY < 0 || candidateY >= len(v.rows) {
				continue
			}
			if v.rows[candidateY].file != v.rows[y].file {
				continue
			}
			if candidate, ok := v.cursorAt(candidateY, pane); ok {
				return candidate, true
			}
			if v.split {
				if candidate, ok := v.cursorAt(candidateY, pane.Other()); ok {
					return candidate, true
				}
			}
		}
	}
	return Cursor{}, false
}

func (v *diffView) JumpFile(cursor Cursor, direction Direction) (Cursor, bool) {
	if !v.valid(cursor) {
		return Cursor{}, false
	}
	file, _ := v.File(cursor)
	y := cursor.Coordinate.Y
	for {
		next, ok := v.scan(y, cursor.Pane, direction, false)
		if !ok {
			return Cursor{}, false
		}
		nextFile, _ := v.File(next)
		if nextFile.OldPath != file.OldPath || nextFile.NewPath != file.NewPath {
			return next, true
		}
		y = next.Coordinate.Y
	}
}

func (v *diffView) SwitchPane(cursor Cursor, pane Pane) (Cursor, bool) {
	if !v.split || !v.valid(cursor) {
		return Cursor{}, false
	}
	if candidate, ok := v.cursorAt(cursor.Coordinate.Y, pane); ok {
		return candidate, true
	}
	for y := cursor.Coordinate.Y - 1; y >= 0; y-- {
		if v.rows[y].file != v.rows[cursor.Coordinate.Y].file {
			break
		}
		if candidate, ok := v.cursorAt(y, pane); ok {
			return candidate, true
		}
	}
	for y := cursor.Coordinate.Y + 1; y < len(v.rows); y++ {
		if candidate, ok := v.cursorAt(y, pane); ok {
			return candidate, true
		}
	}
	return Cursor{}, false
}

func (v *diffView) NewViewport(width, height int) Viewport {
	return v.Resize(Viewport{}, width, height)
}

func (v *diffView) Resize(viewport Viewport, width, height int) Viewport {
	viewport.Width, viewport.Height = max(1, width), max(1, height)
	return v.clampViewport(viewport)
}

func (v *diffView) clampViewport(viewport Viewport) Viewport {
	maxTop := max(0, len(v.rows)-viewport.Height)
	if v.hasStickyHeader(Coordinate{Y: maxTop}, viewport.Height) {
		maxTop++
	}
	viewport.Top.Y = max(0, min(viewport.Top.Y, maxTop))
	viewport.LeftColumn = max(0, min(viewport.LeftColumn, v.maxHorizontalOffset(viewport.Width)))
	return viewport
}

func (v *diffView) hasStickyHeader(top Coordinate, viewportHeight int) bool {
	return viewportHeight > 1 && top.Y >= 0 && top.Y < len(v.rows) && v.rows[top.Y].kind != fileRow
}

func (v *diffView) contentHeight(viewport Viewport) int {
	height := viewport.Height
	if v.hasStickyHeader(viewport.Top, viewport.Height) {
		height--
	}
	return max(1, height)
}

func (v *diffView) KeepVisible(viewport Viewport, cursor Cursor) Viewport {
	if !v.valid(cursor) {
		return v.clampViewport(viewport)
	}
	viewport = v.clampViewport(viewport)
	for range 2 {
		height := v.contentHeight(viewport)
		if cursor.Coordinate.Y < viewport.Top.Y {
			viewport.Top.Y = cursor.Coordinate.Y
		}
		if cursor.Coordinate.Y >= viewport.Top.Y+height {
			viewport.Top.Y = cursor.Coordinate.Y - height + 1
		}
		viewport = v.clampViewport(viewport)
	}
	return viewport
}

func (v *diffView) Align(viewport Viewport, cursor Cursor, alignment VerticalAlignment) Viewport {
	headerHeight := 0
	if viewport.Height > 1 {
		headerHeight = 1
	}
	offset := max(0, alignmentOffset(viewport.Height, alignment)-headerHeight)
	viewport.Top.Y = cursor.Coordinate.Y - offset
	if !v.hasStickyHeader(viewport.Top, viewport.Height) {
		viewport.Top.Y = cursor.Coordinate.Y - alignmentOffset(viewport.Height, alignment)
	}
	return v.clampViewport(viewport)
}

func alignmentOffset(height int, alignment VerticalAlignment) int {
	if alignment == Middle {
		return height / 2
	}
	if alignment == Bottom {
		return height - 1
	}
	return 0
}

func (v *diffView) ScrollHorizontal(viewport Viewport, columns int) Viewport {
	viewport.LeftColumn += columns
	return v.clampViewport(viewport)
}

func (v *diffView) ScrollHalfPage(viewport Viewport, cursor Cursor, direction Direction) (Viewport, Cursor) {
	if !v.valid(cursor) {
		return viewport, cursor
	}
	distance := int(direction) * max(1, viewport.Height/2)
	viewport.Top.Y += distance
	viewport = v.clampViewport(viewport)
	target := min(len(v.rows)-1, max(0, cursor.Coordinate.Y+distance))
	if candidate, ok := v.nearest(target, cursor.Pane, direction, viewport); ok {
		cursor = candidate
	}
	return viewport, cursor
}

func (v *diffView) ViewportProgress(viewport Viewport) int {
	if len(v.rows) == 0 {
		return 0
	}
	bottom := min(len(v.rows), viewport.Top.Y+v.contentHeight(viewport))
	return bottom * 100 / len(v.rows)
}

func (v *diffView) nearest(target int, pane Pane, direction Direction, viewport Viewport) (Cursor, bool) {
	height := v.contentHeight(viewport)
	for distance := 0; distance < height; distance++ {
		for _, y := range []int{target + int(direction)*distance, target - int(direction)*distance} {
			if y < viewport.Top.Y || y >= viewport.Top.Y+height || y >= len(v.rows) {
				continue
			}
			if cursor, ok := v.cursorAt(y, pane); ok {
				return cursor, true
			}
		}
	}
	return Cursor{}, false
}

func (v *diffView) BeginSelection(cursor Cursor) Selection {
	return Selection{First: cursor, Last: cursor}
}

func (v *diffView) ExtendSelection(selection Selection, cursor Cursor) (Selection, bool) {
	if !v.valid(selection.First) || !v.valid(cursor) {
		return selection, false
	}
	first := v.rows[selection.First.Coordinate.Y]
	last := v.rows[cursor.Coordinate.Y]
	if first.file != last.file || first.hunk != last.hunk {
		return selection, false
	}
	selection.Last = cursor
	return selection, true
}

func (v *diffView) Lines(selection Selection) []patch.Line {
	if _, ok := v.ExtendSelection(selection, selection.Last); !ok {
		return nil
	}
	first, last := selection.First.Coordinate.Y, selection.Last.Coordinate.Y
	if first > last {
		first, last = last, first
	}
	lines := make([]patch.Line, 0, last-first+1)
	if first == last && selection.First.Pane != selection.Last.Pane {
		current := v.rows[first]
		indices := []int{v.lineIndex(current, selection.First.Pane), v.lineIndex(current, selection.Last.Pane)}
		for _, index := range indices {
			if index >= 0 && (len(lines) == 0 || lines[len(lines)-1] != v.patch.Files[current.file].Hunks[current.hunk].Lines[index]) {
				lines = append(lines, v.patch.Files[current.file].Hunks[current.hunk].Lines[index])
			}
		}
		return lines
	}
	for y := first; y <= last; y++ {
		pane := selection.First.Pane
		if y == selection.Last.Coordinate.Y {
			pane = selection.Last.Pane
		}
		if line, ok := v.Line(Cursor{Coordinate: Coordinate{Y: y}, Pane: pane}); ok {
			lines = append(lines, line)
		}
	}
	return lines
}

func (v *diffView) Anchor(selection Selection) (review.Anchor, error) {
	lines := v.Lines(selection)
	if len(lines) == 0 {
		return review.Anchor{}, fmt.Errorf("select code lines before commenting")
	}
	first := v.rows[selection.First.Coordinate.Y]
	file := v.patch.Files[first.file]
	hunk := file.Hunks[first.hunk]
	anchor := review.Anchor{FilePath: file.Path()}
	start, end := selection.First.Coordinate.Y, selection.Last.Coordinate.Y
	if start > end {
		start, end = end, start
	}
	for y := start; y <= end; y++ {
		panes := []Pane{selection.First.Pane}
		if start == end && selection.First.Pane != selection.Last.Pane {
			panes = append(panes, selection.Last.Pane)
		} else if y == selection.Last.Coordinate.Y {
			panes[0] = selection.Last.Pane
		}
		for _, pane := range panes {
			index := v.lineIndex(v.rows[y], pane)
			if index < 0 {
				continue
			}
			line := hunk.Lines[index]
			prefix := " "
			if line.Kind == patch.Addition {
				prefix = "+"
			}
			if line.Kind == patch.Deletion {
				prefix = "-"
			}
			anchor.QuotedLines = append(anchor.QuotedLines, prefix+line.Text)
			accumulateRange(&anchor.OldStart, &anchor.OldEnd, int(line.OldNumber))
			accumulateRange(&anchor.NewStart, &anchor.NewEnd, int(line.NewNumber))
		}
	}
	return anchor, nil
}

func (v *diffView) File(cursor Cursor) (patch.File, bool) {
	if !v.valid(cursor) {
		return patch.File{}, false
	}
	return v.patch.Files[v.rows[cursor.Coordinate.Y].file], true
}

func (v *diffView) Hunk(cursor Cursor) (patch.Hunk, bool) {
	if !v.valid(cursor) {
		return patch.Hunk{}, false
	}
	current := v.rows[cursor.Coordinate.Y]
	return v.patch.Files[current.file].Hunks[current.hunk], true
}

func (v *diffView) Line(cursor Cursor) (patch.Line, bool) {
	if !v.valid(cursor) {
		return patch.Line{}, false
	}
	current := v.rows[cursor.Coordinate.Y]
	return v.patch.Files[current.file].Hunks[current.hunk].Lines[v.lineIndex(current, cursor.Pane)], true
}

func (v *diffView) FindCursor(file patch.File, hunk patch.Hunk, line patch.Line, nearby Coordinate, pane Pane) (Cursor, bool) {
	candidates := make([]Cursor, 0)
	fallbacks := make([]Cursor, 0)
	nearbyCandidates := make([]Cursor, 0)
	for y, current := range v.rows {
		if current.file < 0 || !sameFile(v.patch.Files[current.file], file) || current.hunk < 0 || v.patch.Files[current.file].Hunks[current.hunk].Header != hunk.Header {
			continue
		}
		for _, candidatePane := range []Pane{pane, pane.Other()} {
			candidate, ok := v.cursorAt(y, candidatePane)
			if !ok {
				continue
			}
			candidateLine, _ := v.Line(candidate)
			if candidateLine.Kind == line.Kind && candidateLine.OldNumber == line.OldNumber && candidateLine.NewNumber == line.NewNumber {
				return candidate, true
			}
			if candidateLine.Kind == line.Kind && candidateLine.Text == line.Text {
				candidates = append(candidates, candidate)
			}
			if candidateLine.Kind == line.Kind {
				fallbacks = append(fallbacks, candidate)
			}
			nearbyCandidates = append(nearbyCandidates, candidate)
		}
	}
	if len(candidates) > 0 {
		return closest(candidates, nearby), true
	}
	if len(fallbacks) > 0 {
		return closest(fallbacks, nearby), true
	}
	if len(nearbyCandidates) > 0 {
		return closest(nearbyCandidates, nearby), true
	}
	return Cursor{}, false
}

func sameFile(candidate, target patch.File) bool {
	return candidate.OldPath != "" && candidate.OldPath == target.OldPath ||
		candidate.NewPath != "" && candidate.NewPath == target.NewPath
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

func highlightedLine(lines []string, number patch.LineNumber, fallback string) string {
	if number <= 0 || int(number) > len(lines) {
		return fallback
	}
	return lines[int(number)-1]
}

func (v *diffView) maxHorizontalOffset(width int) int {
	contentWidth := max(1, width-14)
	extra := 0
	if v.split {
		contentWidth, extra = max(1, (width-3)/2-6), 2
	}
	longest := 0
	for _, current := range v.rows {
		if current.kind != lineRow {
			continue
		}
		longest = max(longest, lipgloss.Width(expandTabs(ansi.Strip(current.text+current.left+current.right)))+extra)
	}
	return max(0, longest-contentWidth)
}
