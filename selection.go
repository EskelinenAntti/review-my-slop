package main

import (
	"errors"
)

func (s *reviewState) hasChangedLines() bool {
	return len(s.changedLines) > 0
}

func (s *reviewState) current() lineRef {
	if !s.hasChangedLines() {
		return lineRef{}
	}
	return s.changedLines[s.cursor].Ref
}

func (s *reviewState) move(delta int) {
	next := s.cursor + delta
	s.moveTo(next)
}

func (s *reviewState) moveTo(next int) {
	if !s.hasChangedLines() {
		return
	}
	if next < 0 {
		next = 0
	}
	if next >= len(s.changedLines) {
		next = len(s.changedLines) - 1
	}
	desiredSide := s.current().Side
	if s.selectionAnchor != nil {
		desiredSide = s.changedLines[*s.selectionAnchor].Ref.Side
	}
	if ref := s.changedLines[next].sideRef(desiredSide); ref != nil {
		s.changedLines[next].Ref = *ref
	}
	if s.selectionAnchor != nil && !s.canMoveSelectionTo(next) {
		return
	}
	s.cursor = next
	s.message = ""
}

func (s *reviewState) restoreCursor(ref lineRef) {
	if !s.hasChangedLines() {
		s.cursor = 0
		s.top = 0
		return
	}
	if ref.File == "" || ref.Line == 0 {
		s.cursor = 0
		s.top = 0
		return
	}
	best := -1
	for i, changedLine := range s.changedLines {
		if side := changedLine.sideRef(ref.Side); side != nil {
			if side.File == ref.File && side.Line == ref.Line {
				best = i
				break
			}
			if best < 0 && side.File == ref.File {
				best = i
			}
		}
	}
	if best < 0 {
		best = 0
	}
	s.cursor = best
	if ref := s.changedLines[best].sideRef(ref.Side); ref != nil {
		s.changedLines[best].Ref = *ref
	}
}

func (s *reviewState) keepSelectionVisible(rows int) {
	if !s.hasChangedLines() {
		s.top = 0
		return
	}
	bodyRows := max(1, rows-2)
	selectedLine := s.currentDisplayLine()
	if selectedLine < 0 {
		return
	}
	if selectedLine < s.top {
		s.top = selectedLine
	}
	if selectedLine >= s.top+bodyRows {
		s.top = selectedLine - bodyRows + 1
	}
	if s.top < 0 {
		s.top = 0
	}
	if _, ok := s.stickyHeader(); ok && selectedLine == s.top && bodyRows > 1 {
		s.top--
		if s.top < 0 {
			s.top = 0
		}
	}
}

func (s *reviewState) currentDisplayLine() int {
	return s.changedLines[s.cursor].LineIndex
}

func (s *reviewState) toggleSelection() {
	if !s.hasChangedLines() {
		s.message = "No changed line selected."
		return
	}
	if err := s.canSelectRangeError(); err != nil {
		s.message = err.Error()
		return
	}
	if s.selectionAnchor != nil {
		s.clearSelection()
		s.message = "Selection cleared."
		return
	}
	anchor := s.cursor
	s.selectionAnchor = &anchor
	s.message = "Selection started."
}

func (s *reviewState) canSelectRange() bool {
	return s.canSelectRangeError() == nil
}

func (s *reviewState) canSelectRangeError() error {
	if err := s.requireReviewableSource("select multiple lines"); err != nil {
		return err
	}
	if err := s.requirePR("select multiple lines"); err != nil {
		return err
	}
	if err := s.requireDraft("select multiple lines"); err != nil {
		return err
	}
	return nil
}

func (s *reviewState) selectSide(side string) {
	if !s.hasChangedLines() {
		s.message = ""
		return
	}
	changedLine := &s.changedLines[s.cursor]
	ref := changedLine.sideRef(side)
	if ref == nil {
		s.message = ""
		return
	}

	previous := changedLine.Ref
	changedLine.Ref = *ref
	if s.selectionAnchor != nil && !s.selectionRangeValid() {
		changedLine.Ref = previous
		s.message = "Selection must stay within one file and side."
		return
	}
	s.message = ""
}

func (s *reviewState) clearSelection() {
	s.selectionAnchor = nil
}

func (s *reviewState) canMoveSelectionTo(cursor int) bool {
	if s.selectionAnchor == nil {
		return true
	}
	anchor := s.changedLines[*s.selectionAnchor].Ref
	next := s.changedLines[cursor].Ref
	return sameReviewTarget(anchor, next)
}

func (s *reviewState) currentRange() (reviewRange, error) {
	if !s.hasChangedLines() {
		return reviewRange{}, errors.New("No changed line selected.")
	}
	if s.selectionAnchor == nil {
		ref := s.current()
		return reviewRange{Start: ref, End: ref}, nil
	}

	startCursor, endCursor := *s.selectionAnchor, s.cursor
	if startCursor > endCursor {
		startCursor, endCursor = endCursor, startCursor
	}
	start := s.changedLines[startCursor].Ref
	end := s.changedLines[endCursor].Ref
	if !sameReviewTarget(start, end) {
		return reviewRange{}, errors.New("Selection must stay within one file and side.")
	}
	if start.Line > end.Line {
		start, end = end, start
	}
	return reviewRange{Start: start, End: end}, nil
}

func (s *reviewState) displayLineSelection(lineIndex, width int) (changedLine, bool) {
	if s.selectionAnchor == nil {
		return changedLine{}, false
	}
	start, end := *s.selectionAnchor, s.cursor
	if start > end {
		start, end = end, start
	}
	startLine := s.changedLines[start].LineIndex
	endLine := s.changedLines[end].LineIndex
	if startLine > endLine {
		startLine, endLine = endLine, startLine
	}
	if lineIndex < startLine || lineIndex > endLine {
		return changedLine{}, false
	}
	for i := start; i <= end; i++ {
		if s.changedLines[i].LineIndex == lineIndex {
			return s.changedLines[i], true
		}
	}
	changedLine := s.changedLines[s.cursor]
	changedLine.Split = inferredSplit(s.lines[lineIndex], changedLine, width)
	return changedLine, true
}

func (s *reviewState) selectionRangeValid() bool {
	_, err := s.currentRange()
	return err == nil
}

func sameReviewTarget(a, b lineRef) bool {
	return a.File == b.File && a.Side == b.Side
}

func sideLabel(side string) string {
	if side == "old" {
		return "left"
	}
	return "right"
}

func plural(count int, one, many string) string {
	if count == 1 {
		return one
	}
	return many
}

func (s *changedLine) setSideRef(ref lineRef) {
	refCopy := ref
	if ref.Side == "old" {
		s.Left = &refCopy
	} else {
		s.Right = &refCopy
	}
}

func (s changedLine) sideRef(side string) *lineRef {
	if side == "old" {
		return s.Left
	}
	return s.Right
}
