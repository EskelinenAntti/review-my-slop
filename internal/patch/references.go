package patch

import (
	"fmt"
	"path/filepath"
)

type Anchor struct {
	FilePath    string   `json:"file"`
	OldStart    int      `json:"old_start,omitempty"`
	OldEnd      int      `json:"old_end,omitempty"`
	NewStart    int      `json:"new_start,omitempty"`
	NewEnd      int      `json:"new_end,omitempty"`
	QuotedLines []string `json:"quoted_lines"`
}

type Position struct {
	File int
	Hunk int
	Line int
}

type Range struct {
	File  int
	Hunk  int
	Lines []int
}

type Location struct {
	Path string
	Line int
}

func (p Patch) Anchor(r Range) (Anchor, error) {
	if r.File < 0 || r.File >= len(p.Files) {
		return Anchor{}, fmt.Errorf("file index %d is out of range", r.File)
	}
	file := p.Files[r.File]
	if r.Hunk < 0 || r.Hunk >= len(file.Hunks) {
		return Anchor{}, fmt.Errorf("hunk index %d is out of range", r.Hunk)
	}
	if len(r.Lines) == 0 {
		return Anchor{}, fmt.Errorf("select code lines before commenting")
	}
	hunk := file.Hunks[r.Hunk]
	selected := make(map[int]struct{}, len(r.Lines))
	for _, index := range r.Lines {
		if index < 0 || index >= len(hunk.Lines) {
			return Anchor{}, fmt.Errorf("line index %d is out of range", index)
		}
		selected[index] = struct{}{}
	}
	anchor := Anchor{FilePath: file.Path()}
	for index, line := range hunk.Lines {
		if _, ok := selected[index]; !ok {
			continue
		}
		prefix := " "
		if line.Kind == Addition {
			prefix = "+"
		} else if line.Kind == Deletion {
			prefix = "-"
		}
		anchor.QuotedLines = append(anchor.QuotedLines, prefix+line.Text)
		accumulateAnchorRange(&anchor.OldStart, &anchor.OldEnd, int(line.OldNumber))
		accumulateAnchorRange(&anchor.NewStart, &anchor.NewEnd, int(line.NewNumber))
	}
	return anchor, nil
}

func (p Patch) SourceLocation(pos Position) (Location, error) {
	if pos.File < 0 || pos.File >= len(p.Files) {
		return Location{}, fmt.Errorf("file index %d is out of range", pos.File)
	}
	file := p.Files[pos.File]
	if pos.Hunk < 0 || pos.Hunk >= len(file.Hunks) {
		return Location{}, fmt.Errorf("hunk index %d is out of range", pos.Hunk)
	}
	hunk := file.Hunks[pos.Hunk]
	if pos.Line < 0 || pos.Line >= len(hunk.Lines) {
		return Location{}, fmt.Errorf("line index %d is out of range", pos.Line)
	}
	line := hunk.Lines[pos.Line]
	path, number := file.NewPath, line.NewNumber
	if path == "" || path == "/dev/null" {
		path = file.OldPath
	}
	if number == 0 {
		number = line.OldNumber
	}
	if path == "" || path == "/dev/null" || number < 1 {
		return Location{}, fmt.Errorf("current line has no editable working-tree location")
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(p.Repository, filepath.FromSlash(path))
	}
	return Location{Path: path, Line: int(number)}, nil
}

func (p Patch) Counts() (added, removed int) {
	for _, file := range p.Files {
		for _, hunk := range file.Hunks {
			for _, line := range hunk.Lines {
				switch line.Kind {
				case Addition:
					added++
				case Deletion:
					removed++
				}
			}
		}
	}
	return added, removed
}

func accumulateAnchorRange(start, end *int, value int) {
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
