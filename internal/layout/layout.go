// Package layout builds immutable rows and maps between rows and Patch lines.
package layout

import (
	"strings"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

type Format uint8

const (
	Unified Format = iota
	Split
)

type Pane uint8

const (
	Left Pane = iota
	Right
)

func (p Pane) Other() Pane {
	if p == Left {
		return Right
	}
	return Left
}

type Direction int8

const (
	Backward Direction = -1
	Forward  Direction = 1
)

type Cell struct {
	Row  int
	Pane Pane
}
type Selection struct{ First, Last Cell }

type RowKind uint8

const (
	FileRow RowKind = iota
	MetadataRow
	HunkRow
	LineRow
)

type Row struct {
	Kind                RowKind
	File, Hunk          int
	LeftLine, RightLine int
	Text                string
}

type Document struct {
	patch  patch.Patch
	format Format
	rows   []Row
}

func Build(p patch.Patch, format Format) *Document {
	d := &Document{patch: clonePatch(p), format: format}
	for fi, file := range d.patch.Files {
		d.rows = append(d.rows, Row{Kind: FileRow, File: fi, Hunk: -1, LeftLine: -1, RightLine: -1, Text: file.DisplayPath})
		for _, text := range file.Metadata {
			d.rows = append(d.rows, Row{Kind: MetadataRow, File: fi, Hunk: -1, LeftLine: -1, RightLine: -1, Text: text})
		}
		for hi, hunk := range file.Hunks {
			header := hunk.Header
			if !strings.HasPrefix(header, "@@") {
				header = "@@ " + header
			}
			d.rows = append(d.rows, Row{Kind: HunkRow, File: fi, Hunk: hi, LeftLine: -1, RightLine: -1, Text: header})
			if format == Split {
				d.buildSplit(fi, hi, hunk)
			} else {
				for li := range hunk.Lines {
					d.rows = append(d.rows, Row{Kind: LineRow, File: fi, Hunk: hi, LeftLine: li, RightLine: li})
				}
			}
		}
	}
	return d
}

func (d *Document) buildSplit(fi, hi int, hunk patch.Hunk) {
	for i := 0; i < len(hunk.Lines); {
		line := hunk.Lines[i]
		switch line.Kind {
		case patch.Context:
			d.rows = append(d.rows, Row{Kind: LineRow, File: fi, Hunk: hi, LeftLine: i, RightLine: i})
			i++
		case patch.Addition:
			d.rows = append(d.rows, Row{Kind: LineRow, File: fi, Hunk: hi, LeftLine: -1, RightLine: i})
			i++
		case patch.Deletion:
			removedStart := i
			for i < len(hunk.Lines) && hunk.Lines[i].Kind == patch.Deletion {
				i++
			}
			addedStart, addedEnd := i, i
			for addedEnd < len(hunk.Lines) && hunk.Lines[addedEnd].Kind == patch.Addition {
				addedEnd++
			}
			count := max(i-removedStart, addedEnd-addedStart)
			for offset := 0; offset < count; offset++ {
				row := Row{Kind: LineRow, File: fi, Hunk: hi, LeftLine: -1, RightLine: -1}
				if removedStart+offset < i {
					row.LeftLine = removedStart + offset
				}
				if addedStart+offset < addedEnd {
					row.RightLine = addedStart + offset
				}
				d.rows = append(d.rows, row)
			}
			i = addedEnd
		}
	}
}

func (d *Document) Patch() patch.Patch { return clonePatch(d.patch) }
func (d *Document) Format() Format     { return d.format }
func (d *Document) RowCount() int      { return len(d.rows) }
func (d *Document) Row(index int) Row {
	if index < 0 || index >= len(d.rows) {
		return Row{File: -1, Hunk: -1, LeftLine: -1, RightLine: -1}
	}
	return d.rows[index]
}

func (d *Document) Valid(cell Cell) bool { _, ok := d.lineIndex(cell); return ok }
func (d *Document) Position(cell Cell) (patch.Position, bool) {
	line, ok := d.lineIndex(cell)
	if !ok {
		return patch.Position{}, false
	}
	r := d.rows[cell.Row]
	return patch.Position{File: r.File, Hunk: r.Hunk, Line: line}, true
}

func (d *Document) Locate(pos patch.Position, pane Pane) (Cell, bool) {
	if pos.File < 0 || pos.File >= len(d.patch.Files) || pos.Hunk < 0 || pos.Hunk >= len(d.patch.Files[pos.File].Hunks) || pos.Line < 0 || pos.Line >= len(d.patch.Files[pos.File].Hunks[pos.Hunk].Lines) {
		return Cell{}, false
	}
	for i, r := range d.rows {
		if r.Kind == LineRow && r.File == pos.File && r.Hunk == pos.Hunk && d.lineFor(r, pane) == pos.Line {
			return Cell{Row: i, Pane: pane}, true
		}
	}
	return Cell{}, false
}

func (d *Document) Range(s Selection) (patch.Range, error) {
	if !d.Valid(s.First) || !d.Valid(s.Last) {
		return patch.Range{}, errInvalidSelection
	}
	a, b := d.rows[s.First.Row], d.rows[s.Last.Row]
	if a.File != b.File || a.Hunk != b.Hunk {
		return patch.Range{}, errInvalidSelection
	}
	first, last := s.First, s.Last
	if first.Row > last.Row {
		first, last = last, first
	}
	r := patch.Range{File: a.File, Hunk: a.Hunk}
	add := func(row int, pane Pane) {
		if line := d.lineFor(d.rows[row], pane); line >= 0 {
			for _, prev := range r.Lines {
				if prev == line {
					return
				}
			}
			r.Lines = append(r.Lines, line)
		}
	}
	if first.Row == last.Row {
		add(first.Row, first.Pane)
		if s.First.Pane != s.Last.Pane {
			add(first.Row, last.Pane)
		}
	} else {
		for row := first.Row; row <= last.Row; row++ {
			pane := s.First.Pane
			if s.First.Row > s.Last.Row {
				pane = s.Last.Pane
			}
			if row == s.Last.Row {
				pane = s.Last.Pane
			} else if row == s.First.Row {
				pane = s.First.Pane
			}
			add(row, pane)
		}
	}
	if len(r.Lines) == 0 {
		return patch.Range{}, errInvalidSelection
	}
	return r, nil
}

func (d *Document) Find(text string, from Cell, direction Direction) (Cell, bool) {
	if text == "" || !d.Valid(from) {
		return Cell{}, false
	}
	query := strings.ToLower(text)
	for count := 0; count < len(d.rows)-1; count++ {
		y := from.Row + int(direction)*(count+1)
		for y < 0 {
			y += len(d.rows)
		}
		for y >= len(d.rows) {
			y -= len(d.rows)
		}
		r := d.rows[y]
		for _, pane := range []Pane{from.Pane, from.Pane.Other()} {
			cell := Cell{Row: y, Pane: pane}
			if !d.Valid(cell) || d.format != Split && pane != from.Pane {
				continue
			}
			line, _ := d.line(cell)
			if strings.Contains(strings.ToLower(line.Text), query) {
				return cell, true
			}
		}
		if r.Kind != LineRow && strings.Contains(strings.ToLower(r.Text), query) {
			if c, ok := d.nearRow(y, from.Pane, direction); ok {
				return c, true
			}
		}
	}
	return Cell{}, false
}

var errInvalidSelection = selectionError("selection must contain lines from one hunk")

type selectionError string

func (e selectionError) Error() string { return string(e) }

func (d *Document) lineIndex(cell Cell) (int, bool) {
	if cell.Row < 0 || cell.Row >= len(d.rows) {
		return -1, false
	}
	r := d.rows[cell.Row]
	if r.Kind != LineRow {
		return -1, false
	}
	i := d.lineFor(r, cell.Pane)
	return i, i >= 0
}
func (d *Document) lineFor(r Row, pane Pane) int {
	if d.format == Unified || pane == Right {
		return r.RightLine
	}
	return r.LeftLine
}
func (d *Document) line(cell Cell) (patch.Line, bool) {
	i, ok := d.lineIndex(cell)
	if !ok {
		return patch.Line{}, false
	}
	r := d.rows[cell.Row]
	return d.patch.Files[r.File].Hunks[r.Hunk].Lines[i], true
}
func (d *Document) nearRow(y int, pane Pane, direction Direction) (Cell, bool) {
	for distance := 1; distance <= len(d.rows); distance++ {
		for _, row := range []int{y + int(direction)*distance, y - int(direction)*distance} {
			if row < 0 || row >= len(d.rows) || d.rows[row].File != d.rows[y].File {
				continue
			}
			if c := (Cell{Row: row, Pane: pane}); d.Valid(c) {
				return c, true
			}
			if d.format == Split {
				if c := (Cell{Row: row, Pane: pane.Other()}); d.Valid(c) {
					return c, true
				}
			}
		}
	}
	return Cell{}, false
}

func clonePatch(p patch.Patch) patch.Patch {
	copyPatch := p
	copyPatch.Files = append([]patch.File(nil), p.Files...)
	for i := range copyPatch.Files {
		copyPatch.Files[i].Metadata = append([]string(nil), p.Files[i].Metadata...)
		copyPatch.Files[i].Hunks = append([]patch.Hunk(nil), p.Files[i].Hunks...)
		for j := range copyPatch.Files[i].Hunks {
			copyPatch.Files[i].Hunks[j].Lines = append([]patch.Line(nil), p.Files[i].Hunks[j].Lines...)
		}
	}
	return copyPatch
}
