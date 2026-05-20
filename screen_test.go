package main

import (
	"strings"
	"testing"
)

type testCell struct {
	ch      byte
	inverse bool
	dim     bool
	fg      string
}

type testScreen struct {
	rows  int
	cols  int
	row   int
	col   int
	cells [][]testCell
	style testCell
}

func newTestScreen(rows, cols int) *testScreen {
	s := &testScreen{
		rows: rows,
		cols: cols,
	}
	s.cells = make([][]testCell, rows)
	for row := range rows {
		s.cells[row] = make([]testCell, cols)
		for col := range cols {
			s.cells[row][col].ch = ' '
		}
	}
	return s
}

func parseTestScreen(t *testing.T, rows, cols int, stream string) *testScreen {
	t.Helper()

	s := newTestScreen(rows, cols)
	for i := 0; i < len(stream); {
		switch stream[i] {
		case '\x1b':
			end := ansiEnd(stream, i)
			if end < 0 {
				t.Fatalf("unterminated escape sequence at byte %d in %q", i, stream[i:])
			}
			s.applyEscape(stream[i:end])
			i = end
		case '\r':
			s.col = 0
			i++
		case '\n':
			if s.row < s.rows-1 {
				s.row++
			}
			i++
		default:
			s.put(stream[i])
			i++
		}
	}
	return s
}

func (s *testScreen) applyEscape(seq string) {
	switch {
	case seq == "\x1b[H":
		s.row = 0
		s.col = 0
	case seq == "\x1b[2J":
		for row := range s.rows {
			for col := range s.cols {
				s.cells[row][col] = testCell{ch: ' '}
			}
		}
		s.row = 0
		s.col = 0
	case seq == "\x1b[K":
		if s.row >= s.rows {
			return
		}
		for col := s.col; col < s.cols; col++ {
			s.cells[s.row][col] = testCell{ch: ' '}
		}
	case strings.HasSuffix(seq, "m"):
		s.applySGR(seq)
	}
}

func (s *testScreen) applySGR(seq string) {
	params := strings.TrimSuffix(strings.TrimPrefix(seq, "\x1b["), "m")
	if params == "" {
		params = "0"
	}
	for _, param := range strings.Split(params, ";") {
		switch param {
		case "0":
			s.style = testCell{}
		case "2":
			s.style.dim = true
		case "7":
			s.style.inverse = true
		case "31", "91":
			s.style.fg = "red"
		case "32", "92":
			s.style.fg = "green"
		}
	}
}

func (s *testScreen) put(ch byte) {
	if s.row >= s.rows || s.col >= s.cols {
		return
	}
	cell := s.style
	cell.ch = ch
	s.cells[s.row][s.col] = cell
	s.col++
}

func (s *testScreen) line(row int) string {
	var b strings.Builder
	for _, cell := range s.cells[row] {
		b.WriteByte(cell.ch)
	}
	return b.String()
}

func (s *testScreen) inverseRange(row int) (int, int, bool) {
	start := -1
	end := -1
	for col, cell := range s.cells[row] {
		if cell.inverse {
			if start < 0 {
				start = col
			}
			end = col + 1
		}
	}
	return start, end, start >= 0
}

func renderScreen(t *testing.T, state *reviewState, rows, cols int) *testScreen {
	t.Helper()

	var out strings.Builder
	render(&out, state, rows, cols)
	return parseTestScreen(t, rows, cols, out.String())
}

func TestRenderScreenShowsCursorOnSelectedSideOnly(t *testing.T) {
	left := lineRef{File: "a.go", Line: 10, Side: "old", Content: "removed"}
	right := lineRef{File: "a.go", Line: 10, Side: "new", Content: "added"}
	state := &reviewState{
		lines: []string{
			"\x1b[91m 10 removed\x1b[0m       \x1b[92m 10 added\x1b[0m",
		},
		selections: []displaySelection{{
			LineIndex: 0,
			Ref:       right,
			Left:      &left,
			Right:     &right,
			Split:     18,
		}},
	}

	screen := renderScreen(t, state, 8, 40)
	start, end, ok := screen.inverseRange(0)
	if !ok {
		t.Fatal("expected selected row to contain inverse cells")
	}
	if start != 18 || end != 40 {
		t.Fatalf("inverse range = %d-%d, want 18-40; line = %q", start, end, screen.line(0))
	}
	for col := 0; col < 18; col++ {
		if screen.cells[0][col].inverse {
			t.Fatalf("left side column %d was inverse; line = %q", col, screen.line(0))
		}
	}
}

func TestRenderScreenClearsStaleContentBetweenFrames(t *testing.T) {
	state := &reviewState{
		lines: []string{
			"short",
		},
		selections: []displaySelection{
			testSelection(lineRef{File: "a.go", Line: 1, Side: "new", Content: "short"}),
		},
		message: "ok",
	}

	var out strings.Builder
	out.WriteString("leftover text that should disappear")
	render(&out, state, 8, 40)

	screen := parseTestScreen(t, 8, 40, out.String())
	if strings.Contains(screen.line(0), "leftover") {
		t.Fatalf("stale content remained on first row: %q", screen.line(0))
	}
	if got := strings.TrimRight(screen.line(1), " "); got != "" {
		t.Fatalf("row after short line = %q, want cleared blank row", got)
	}
}
