package tui

import (
	"strings"
	"testing"
)

type screenCell struct {
	ch      byte
	inverse bool
	fg      string
}

func TestRenderHighlightsSelectedLineAcrossFullWidthWithANSIInput(t *testing.T) {
	var out strings.Builder
	Render(&out, []string{"plain", "\x1b[92madded\x1b[0m tail"}, Viewport{Cursor: 1, Rows: 2, Total: 2}, 12)

	screen := parseScreen(t, out.String(), 2, 12)
	for col, cell := range screen[1] {
		if !cell.inverse {
			t.Fatalf("column %d was not inverse; row = %#v", col, screen[1])
		}
		if cell.fg != "" {
			t.Fatalf("column %d had fg %q under cursor; row = %#v", col, cell.fg, screen[1])
		}
	}
}

func TestRenderStickyLineKeepsTopContentVisibleBelow(t *testing.T) {
	var out strings.Builder
	RenderWithOptions(&out, []string{"header", "first", "second"}, Viewport{Top: 1, Cursor: 1, Rows: 3, Total: 3}, 12, RenderOptions{
		Sticky: StickyLine{Text: "header", Active: true},
	})

	screen := parseScreen(t, out.String(), 3, 12)
	if got := screenLine(screen[0]); got != "header" {
		t.Fatalf("row 0 = %q, want header", got)
	}
	if got := strings.TrimRight(screenLine(screen[1]), " "); got != "first" {
		t.Fatalf("row 1 = %q, want first", got)
	}
	for col, cell := range screen[1] {
		if !cell.inverse {
			t.Fatalf("row 1 column %d was not inverse", col)
		}
	}
}

func screenLine(screen []screenCell) string {
	var out strings.Builder
	for _, cell := range screen {
		out.WriteByte(cell.ch)
	}
	return strings.TrimRight(out.String(), " ")
}

func parseScreen(t *testing.T, stream string, rows, cols int) [][]screenCell {
	t.Helper()
	screen := make([][]screenCell, rows)
	for row := range screen {
		screen[row] = make([]screenCell, cols)
		for col := range screen[row] {
			screen[row][col].ch = ' '
		}
	}
	row, col := 0, 0
	inverse := false
	fg := ""
	for i := 0; i < len(stream); {
		switch stream[i] {
		case '\x1b':
			end := ansiEnd(stream, i)
			if end < 0 {
				t.Fatalf("unterminated escape at %d", i)
			}
			seq := stream[i:end]
			if seq == "\x1b[7m" {
				inverse = true
			}
			if seq == "\x1b[0m" {
				inverse = false
				fg = ""
			}
			if seq == "\x1b[31m" || seq == "\x1b[91m" {
				fg = "red"
			}
			if seq == "\x1b[32m" || seq == "\x1b[92m" {
				fg = "green"
			}
			if seq == "\x1b[H" {
				row, col = 0, 0
			}
			i = end
		case '\r':
			col = 0
			i++
		case '\n':
			if row < rows-1 {
				row++
			}
			i++
		default:
			if row < rows && col < cols {
				screen[row][col] = screenCell{ch: stream[i], inverse: inverse, fg: fg}
				col++
			}
			i++
		}
	}
	return screen
}
