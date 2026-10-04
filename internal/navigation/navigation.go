// Package navigation owns mutable cursor, selection and viewport state.
package navigation

import (
	"strings"

	"github.com/eskelinenantti/review-my-slop/internal/layout"
	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/mattn/go-runewidth"
)

type Alignment uint8

const (
	Top Alignment = iota
	Middle
	Bottom
)

type Viewport struct{ Top, LeftColumn, Width, Height int }
type Snapshot struct {
	Cursor    *layout.Cell
	Selection *layout.Selection
	Viewport  Viewport
}

type Navigation struct {
	doc         *layout.Document
	cursor      *layout.Cell
	selection   *layout.Selection
	viewport    Viewport
	longestLine int
}

func New(d *layout.Document, width, height int) *Navigation {
	n := &Navigation{doc: d, viewport: Viewport{Width: max(1, width), Height: max(1, height)}}
	if d != nil {
		n.longestLine = plainLongestLine(d)
		if cell, ok := first(d, layout.Right); ok {
			n.cursor = cell
		} else if cell, ok := first(d, layout.Left); ok {
			n.cursor = cell
		}
	}
	n.viewport = n.clamp(n.viewport)
	return n
}

func (n *Navigation) Move(direction layout.Direction) {
	if n.cursor == nil || n.doc == nil {
		return
	}
	for row := n.cursor.Row + int(direction); row >= 0 && row < n.doc.RowCount(); row += int(direction) {
		candidate := layout.Cell{Row: row, Pane: n.cursor.Pane}
		if n.doc.Valid(candidate) {
			n.jump(candidate)
			return
		}
	}
}
func (n *Navigation) First() {
	if n.doc == nil {
		return
	}
	if c, ok := first(n.doc, layout.Right); ok {
		n.jump(*c)
		return
	}
	if c, ok := first(n.doc, layout.Left); ok {
		n.jump(*c)
	}
}
func (n *Navigation) Last() {
	if n.doc == nil {
		return
	}
	if c, ok := last(n.doc, layout.Right); ok {
		n.jump(*c)
		return
	}
	if c, ok := last(n.doc, layout.Left); ok {
		n.jump(*c)
	}
}
func (n *Navigation) Jump(cell layout.Cell) { n.jump(cell) }

func (n *Navigation) JumpFile(direction layout.Direction) {
	n.selection = nil
	if n.doc == nil || n.cursor == nil {
		return
	}
	current := n.doc.Row(n.cursor.Row).File
	for row := n.cursor.Row + int(direction); row >= 0 && row < n.doc.RowCount(); row += int(direction) {
		candidate := layout.Cell{Row: row, Pane: n.cursor.Pane}
		if n.doc.Valid(candidate) && n.doc.Row(row).File != current {
			n.jump(candidate)
			return
		}
	}
}
func (n *Navigation) SwitchPane(pane layout.Pane) {
	if n.doc == nil || n.cursor == nil || n.doc.Format() != layout.Split {
		return
	}
	candidate, ok := n.switchCell(*n.cursor, pane)
	if !ok {
		return
	}
	var translated *layout.Selection
	if n.selection != nil {
		first, firstOK := n.switchCell(n.selection.First, pane)
		last, lastOK := n.switchCell(n.selection.Last, pane)
		if !firstOK || !lastOK {
			return
		}
		s := layout.Selection{First: first, Last: last}
		if _, err := n.doc.Range(s); err != nil {
			return
		}
		translated = &s
	}
	n.selection = translated
	cursor := candidate
	n.cursor = &cursor
	n.keepVisible(candidate)
}
func (n *Navigation) BeginSelection() {
	if n.cursor == nil {
		return
	}
	s := layout.Selection{First: *n.cursor, Last: *n.cursor}
	n.selection = &s
}
func (n *Navigation) CancelSelection() { n.selection = nil }
func (n *Navigation) ScrollHorizontal(columns int) {
	n.viewport.LeftColumn += columns
	n.viewport = n.clamp(n.viewport)
}
func (n *Navigation) HalfPage(direction layout.Direction) {
	if n.doc == nil || n.cursor == nil {
		return
	}
	distance := int(direction) * max(1, n.viewport.Height/2)
	viewport := n.viewport
	viewport.Top += distance
	viewport = n.clamp(viewport)
	target := min(n.doc.RowCount()-1, max(0, n.cursor.Row+distance))
	height := n.contentHeight(viewport)
	for d := 0; d < height; d++ {
		for _, y := range []int{target + int(direction)*d, target - int(direction)*d} {
			if y < viewport.Top || y >= viewport.Top+height {
				continue
			}
			c := layout.Cell{Row: y, Pane: n.cursor.Pane}
			if n.doc.Valid(c) {
				selection := n.selection
				if selection != nil {
					s := *selection
					s.Last = c
					if _, err := n.doc.Range(s); err != nil {
						return
					}
					selection = &s
				}
				n.selection = selection
				cursor := c
				n.cursor = &cursor
				n.viewport = viewport
				return
			}
		}
	}
}
func (n *Navigation) Align(alignment Alignment) {
	if n.cursor == nil {
		return
	}
	headerHeight := 0
	if n.viewport.Height > 1 {
		headerHeight = 1
	}
	offset := max(0, alignmentOffset(n.viewport.Height, alignment)-headerHeight)
	n.viewport.Top = n.cursor.Row - offset
	if !n.hasSticky(n.viewport.Top, n.viewport.Height) {
		n.viewport.Top = n.cursor.Row - alignmentOffset(n.viewport.Height, alignment)
	}
	n.viewport = n.clamp(n.viewport)
}

func alignmentOffset(height int, alignment Alignment) int {
	switch alignment {
	case Middle:
		return height / 2
	case Bottom:
		return height - 1
	default:
		return 0
	}
}
func (n *Navigation) Resize(width, height int) {
	n.viewport.Width = max(1, width)
	n.viewport.Height = max(1, height)
	n.viewport = n.clamp(n.viewport)
	if n.cursor != nil {
		n.keepVisible(*n.cursor)
	}
}

func (n *Navigation) Replace(d *layout.Document) {
	oldDoc, oldCursor, oldSelection, oldViewport := n.doc, n.cursor, n.selection, n.viewport
	rowsAbove := 0
	if oldCursor != nil {
		rowsAbove = oldCursor.Row - oldViewport.Top
	}
	n.doc = d
	n.cursor = nil
	n.selection = nil
	n.longestLine = 0
	if d != nil {
		n.longestLine = plainLongestLine(d)
		if oldDoc != nil && oldCursor != nil {
			if translated, ok := preserveCell(oldDoc, d, *oldCursor); ok {
				n.cursor = &translated
			}
		}
		if n.cursor == nil {
			if c, ok := first(d, layout.Right); ok {
				n.cursor = c
			} else if c, ok := first(d, layout.Left); ok {
				n.cursor = c
			}
		}
		if oldDoc != nil && oldSelection != nil {
			if a, ok := preserveCell(oldDoc, d, oldSelection.First); ok {
				if b, ok := preserveCell(oldDoc, d, oldSelection.Last); ok {
					s := layout.Selection{First: a, Last: b}
					if _, err := d.Range(s); err == nil {
						n.selection = &s
					}
				}
			}
		}
	}
	n.viewport.Width = max(1, oldViewport.Width)
	n.viewport.Height = max(1, oldViewport.Height)
	n.viewport.LeftColumn = oldViewport.LeftColumn
	n.viewport.Top = 0
	if n.cursor != nil {
		n.viewport.Top = n.cursor.Row - rowsAbove
	}
	n.viewport = n.clamp(n.viewport)
	if n.cursor != nil {
		n.keepVisible(*n.cursor)
	}
}
func (n *Navigation) Snapshot() Snapshot {
	s := Snapshot{Viewport: n.viewport}
	if n.cursor != nil {
		c := *n.cursor
		s.Cursor = &c
	}
	if n.selection != nil {
		v := *n.selection
		s.Selection = &v
	}
	return s
}
func (n *Navigation) Progress() int {
	if n.doc == nil || n.doc.RowCount() == 0 {
		return 0
	}
	bottom := min(n.doc.RowCount(), n.viewport.Top+n.contentHeight(n.viewport))
	return bottom * 100 / n.doc.RowCount()
}

func (n *Navigation) jump(c layout.Cell) {
	if n.doc == nil || !n.doc.Valid(c) {
		return
	}
	if n.selection != nil {
		s := *n.selection
		s.Last = c
		if _, err := n.doc.Range(s); err != nil {
			return
		}
		n.selection = &s
	}
	v := c
	n.cursor = &v
	n.keepVisible(c)
}
func (n *Navigation) clamp(v Viewport) Viewport {
	if n.doc == nil || n.doc.RowCount() == 0 {
		v.Top = 0
		v.LeftColumn = 0
		return v
	}
	maxTop := max(0, n.doc.RowCount()-v.Height)
	if n.hasSticky(maxTop, v.Height) {
		maxTop++
	}
	v.Top = max(0, min(v.Top, maxTop))
	v.LeftColumn = max(0, min(v.LeftColumn, n.maxHorizontalOffset(v.Width)))
	return v
}
func (n *Navigation) hasSticky(top, height int) bool {
	return n.doc != nil && height > 1 && top >= 0 && top < n.doc.RowCount() && n.doc.Row(top).Kind != layout.FileRow
}
func (n *Navigation) contentHeight(v Viewport) int {
	h := v.Height
	if n.hasSticky(v.Top, v.Height) {
		h--
	}
	return max(1, h)
}
func (n *Navigation) keepVisible(c layout.Cell) {
	for range 2 {
		h := n.contentHeight(n.viewport)
		if c.Row < n.viewport.Top {
			n.viewport.Top = c.Row
		}
		if c.Row >= n.viewport.Top+h {
			n.viewport.Top = c.Row - h + 1
		}
		n.viewport = n.clamp(n.viewport)
	}
}
func (n *Navigation) maxHorizontalOffset(width int) int {
	if n.doc == nil {
		return 0
	}
	contentWidth := max(1, width-14)
	extra := 0
	if n.doc.Format() == layout.Split {
		contentWidth, extra = max(1, (width-3)/2-6), 2
	}
	return max(0, n.longestLine+extra-contentWidth)
}

func plainLongestLine(d *layout.Document) int {
	longest := 0
	p := d.Patch()
	for i := 0; i < d.RowCount(); i++ {
		r := d.Row(i)
		if r.Kind != layout.LineRow {
			continue
		}
		for _, index := range []int{r.LeftLine, r.RightLine} {
			if index < 0 {
				continue
			}
			line := p.Files[r.File].Hunks[r.Hunk].Lines[index]
			longest = max(longest, runewidth.StringWidth(strings.ReplaceAll(line.Text, "\t", "    ")))
		}
	}
	return longest
}

func (n *Navigation) switchCell(origin layout.Cell, pane layout.Pane) (layout.Cell, bool) {
	if c := (layout.Cell{Row: origin.Row, Pane: pane}); n.doc.Valid(c) {
		return c, true
	}
	file := n.doc.Row(origin.Row).File
	for y := origin.Row - 1; y >= 0 && n.doc.Row(y).File == file; y-- {
		if c := (layout.Cell{Row: y, Pane: pane}); n.doc.Valid(c) {
			return c, true
		}
	}
	for y := origin.Row + 1; y < n.doc.RowCount(); y++ {
		if c := (layout.Cell{Row: y, Pane: pane}); n.doc.Valid(c) {
			return c, true
		}
	}
	return layout.Cell{}, false
}

func first(d *layout.Document, p layout.Pane) (*layout.Cell, bool) {
	for i := 0; i < d.RowCount(); i++ {
		c := layout.Cell{Row: i, Pane: p}
		if d.Valid(c) {
			return &c, true
		}
	}
	return nil, false
}
func last(d *layout.Document, p layout.Pane) (*layout.Cell, bool) {
	for i := d.RowCount() - 1; i >= 0; i-- {
		c := layout.Cell{Row: i, Pane: p}
		if d.Valid(c) {
			return &c, true
		}
	}
	return nil, false
}

func preserveCell(oldDoc, newDoc *layout.Document, c layout.Cell) (layout.Cell, bool) {
	pos, ok := oldDoc.Position(c)
	if !ok {
		return layout.Cell{}, false
	}
	oldPatch, newPatch := oldDoc.Patch(), newDoc.Patch()
	oldRow := oldDoc.Row(c.Row)
	oldFile := oldPatch.Files[oldRow.File]
	oldHunk := oldFile.Hunks[oldRow.Hunk]
	oldLine := oldHunk.Lines[pos.Line]
	var numbered, exact, sameKind, nearby []layout.Cell
	for rowIndex := 0; rowIndex < newDoc.RowCount(); rowIndex++ {
		row := newDoc.Row(rowIndex)
		if row.Kind != layout.LineRow {
			continue
		}
		fi := row.File
		file := newPatch.Files[fi]
		if !samePath(oldFile, file) {
			continue
		}
		hunk := file.Hunks[row.Hunk]
		if hunk.Header != oldHunk.Header {
			continue
		}
		for _, pane := range []layout.Pane{c.Pane, c.Pane.Other()} {
			lineIndex := row.RightLine
			if pane == layout.Left && newDoc.Format() == layout.Split {
				lineIndex = row.LeftLine
			}
			if lineIndex < 0 {
				continue
			}
			line := hunk.Lines[lineIndex]
			cell := layout.Cell{Row: rowIndex, Pane: pane}
			nearby = append(nearby, cell)
			if line.Kind == oldLine.Kind {
				sameKind = append(sameKind, cell)
			}
			if line.Kind == oldLine.Kind && line.Text == oldLine.Text {
				exact = append(exact, cell)
			}
			if line.Kind == oldLine.Kind && line.OldNumber == oldLine.OldNumber && line.NewNumber == oldLine.NewNumber {
				numbered = append(numbered, cell)
			}
		}
	}
	for _, candidates := range [][]layout.Cell{numbered, exact, sameKind, nearby} {
		if len(candidates) > 0 {
			return closestCell(candidates, c.Row), true
		}
	}
	return layout.Cell{}, false
}

func closestCell(candidates []layout.Cell, row int) layout.Cell {
	best := candidates[0]
	for _, candidate := range candidates[1:] {
		if abs(candidate.Row-row) < abs(best.Row-row) {
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
func samePath(a, b patch.File) bool {
	return a.OldPath != "" && a.OldPath == b.OldPath || a.NewPath != "" && a.NewPath == b.NewPath
}
