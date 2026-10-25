package diffscreen

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

func TestCodeLineWindowKeepsGutterFixed(t *testing.T) {
	const unifiedGutter = "          7 + "
	for _, tt := range []struct {
		name          string
		split         bool
		width, offset int
		want          string
	}{
		{"unified marker stays fixed", false, 18, 2, unifiedGutter + "cdef"},
		{"split marker scrolls", true, 10, 2, "    7 abcd"},
		{"unified padding", false, 24, 4, unifiedGutter + "efghij    "},
		{"split padding", true, 18, 6, "    7 efghij      "},
		{"unified narrow", false, 10, 2, strings.Repeat(" ", 10)},
		{"split narrow", true, 4, 2, "    "},
	} {
		t.Run(tt.name, func(t *testing.T) {
			p := patch.Patch{Files: []patch.File{{Hunks: []patch.Hunk{{
				Lines: []patch.Line{{
					Kind: patch.Addition, NewNumber: 7, Text: "abcdefghij",
				}},
			}}}}}
			v := newDiffView(p, Options{SideBySide: tt.split})
			cursor, ok := v.first()
			if !ok {
				t.Fatal("missing code line")
			}
			row := v.rows[cursor.row]
			text := "\x1b[32mabcdefghij\x1b[0m"
			row.text, row.right = text, text
			var rendered string
			if tt.split {
				rendered = v.renderPane(
					row, cursor.row, right, tt.width, tt.offset, cursor, nil,
				)
			} else {
				viewport := diffViewport{leftColumn: tt.offset}
				rendered = v.renderUnifiedLine(
					row, cursor.row, viewport, cursor, nil, tt.width,
				)
			}
			if got := ansi.Strip(rendered); got != tt.want {
				t.Errorf("rendered = %q, want %q", got, tt.want)
			}
		})
	}
}
