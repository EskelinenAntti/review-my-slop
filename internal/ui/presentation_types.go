package ui

import (
	"errors"

	"github.com/eskelinenantti/review-my-slop/internal/comments"
	"github.com/eskelinenantti/review-my-slop/internal/diff"
)

// Coordinate is a row in the current terminal presentation.
type Coordinate struct{ Y int }

// Pane identifies one side of a side-by-side presentation.
type Pane uint8

const (
	Left Pane = iota
	Right
)

func (pane Pane) Other() Pane {
	if pane == Left {
		return Right
	}
	return Left
}

// Cursor is presentation state. It is intentionally not stored in the diff
// domain and must be translated after a layout or content rebuild.
type Cursor struct {
	Coordinate Coordinate
	Pane       Pane
}

// Viewport is the visible terminal window over presentation rows.
type Viewport struct {
	Top        Coordinate
	LeftColumn int
	Width      int
	Height     int
}

// Selection is a range of presentation cursors within one hunk.
type Selection struct {
	First Cursor
	Last  Cursor
}

// Direction is a navigation direction.
type Direction int8

const (
	Backward Direction = -1
	Forward  Direction = 1
)

// VerticalAlignment controls where a cursor is positioned in the viewport.
type VerticalAlignment uint8

const (
	Top VerticalAlignment = iota
	Middle
	Bottom
)

type rowKind uint8

const (
	fileRow rowKind = iota
	metadataRow
	hunkRow
	lineRow
)

type row struct {
	kind                 rowKind
	fileIndex, hunkIndex int
	leftLine, rightLine  int
	text, left, right    string
	hunkHeader           string
}

type highlightedSource struct {
	old []string
	new []string
}

type presentation struct {
	changes diff.ChangeSet
	rows    []row
	split   bool
	dark    bool
}

// lineIdentity is the semantic state needed to restore a cursor after a
// refresh. Coordinate is only a proximity hint for duplicate lines.
type lineIdentity struct {
	file   diff.File
	hunk   diff.Hunk
	line   diff.Line
	cursor Cursor
	valid  bool
}

func (p *presentation) identify(cursor Cursor) lineIdentity {
	file, fileOK := p.file(cursor)
	hunk, hunkOK := p.hunk(cursor)
	line, lineOK := p.line(cursor)
	return lineIdentity{file: file, hunk: hunk, line: line, cursor: cursor, valid: fileOK && hunkOK && lineOK}
}

func (p *presentation) newView(changes diff.ChangeSet, split, dark bool) {
	p.changes = changes
	p.rows = nil
	p.split = split
	p.dark = dark
	if split {
		p.buildSplit()
		return
	}
	p.buildUnified()
}

func (p *presentation) file(cursor Cursor) (diff.File, bool) {
	if !p.valid(cursor) {
		return diff.File{}, false
	}
	return p.changes.Files[p.rows[cursor.Coordinate.Y].fileIndex], true
}

func (p *presentation) hunk(cursor Cursor) (diff.Hunk, bool) {
	if !p.valid(cursor) {
		return diff.Hunk{}, false
	}
	current := p.rows[cursor.Coordinate.Y]
	return p.changes.Files[current.fileIndex].Hunks[current.hunkIndex], true
}

func (p *presentation) line(cursor Cursor) (diff.Line, bool) {
	if !p.valid(cursor) {
		return diff.Line{}, false
	}
	current := p.rows[cursor.Coordinate.Y]
	index := p.lineIndex(current, cursor.Pane)
	if index < 0 {
		return diff.Line{}, false
	}
	return p.changes.Files[current.fileIndex].Hunks[current.hunkIndex].Lines[index], true
}

func (p *presentation) anchor(selection Selection) (comments.Anchor, error) {
	selected, file, ok := p.selectedLines(selection)
	if !ok || len(selected) == 0 {
		return comments.Anchor{}, errors.New("select code lines before commenting")
	}
	anchor := comments.Anchor{FilePath: file.Path()}
	for _, line := range selected {
		prefix := " "
		if line.Kind == diff.Addition {
			prefix = "+"
		}
		if line.Kind == diff.Deletion {
			prefix = "-"
		}
		anchor.QuotedLines = append(anchor.QuotedLines, prefix+line.Text)
		accumulateRange(&anchor.OldStart, &anchor.OldEnd, int(line.OldNumber))
		accumulateRange(&anchor.NewStart, &anchor.NewEnd, int(line.NewNumber))
	}
	return anchor, nil
}
