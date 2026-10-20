package diffscreen

import (
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestProjectedLineLookup(t *testing.T) {
	deleted := patch.Line{Kind: patch.Deletion, Text: "old", OldNumber: 1}
	added := patch.Line{Kind: patch.Addition, Text: "new", NewNumber: 1}
	extra := patch.Line{Kind: patch.Addition, Text: "extra", NewNumber: 2}
	p := patchWithLines([]patch.Line{deleted, added, extra})
	for _, tc := range []struct {
		name  string
		split bool
		rows  [][2]patch.Line
	}{
		{"unified", false, [][2]patch.Line{
			{deleted, deleted}, {added, added}, {extra, extra},
		}},
		{"split", true, [][2]patch.Line{
			{deleted, added}, {{}, extra},
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := newDiffView(p, Options{SideBySide: tc.split})
			lineRowIndex := 0
			for y, row := range v.rows {
				for _, pane := range []diffPane{left, right} {
					var want patch.Line
					if row.kind == lineRow {
						want = tc.rows[lineRowIndex][pane]
					}
					cursor := diffCursor{row: y, pane: pane}
					got, ok := v.line(cursor)
					testutil.RequireEqual(t, got, want,
						"row %d pane %d", y, pane)
					testutil.RequireEqual(t, ok, want != (patch.Line{}),
						"row %d pane %d validity", y, pane)
					testutil.RequireEqual(t, ok, v.valid(cursor),
						"row %d pane %d cursor validity", y, pane)
				}
				if row.kind == lineRow {
					lineRowIndex++
				}
			}
			testutil.RequireEqual(t, lineRowIndex, len(tc.rows),
				"line row count")
			for _, y := range []int{-1, len(v.rows)} {
				got, ok := v.line(diffCursor{row: y, pane: right})
				testutil.RequireEqual(t, got, patch.Line{}, "out-of-range line")
				testutil.RequireEqual(t, ok, false, "out-of-range validity")
			}
		})
	}
}
