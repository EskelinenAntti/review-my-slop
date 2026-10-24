package diffscreen

import (
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestSourceLookup(t *testing.T) {
	p := testPatch()
	p.Files[0].Metadata = []string{"mode changed"}
	p.Files[0].Hunks = append(p.Files[0].Hunks, patch.Hunk{
		Header: "@@ -10 +10 @@",
		Lines: []patch.Line{{
			Kind: patch.Context, Text: "next hunk",
			OldNumber: 10, NewNumber: 10,
		}},
	})
	for _, layout := range []struct {
		name  string
		split bool
	}{{"unified", false}, {"split", true}} {
		t.Run(layout.name, func(t *testing.T) {
			v := newDiffView(p, Options{SideBySide: layout.split})
			for _, tc := range []struct {
				name                 string
				unifiedRow, splitRow int
				pane                 diffPane
				file, hunk, line     int
			}{
				{"before rows", -1, -1, right, 0, 0, -1},
				{"file header", 0, 0, right, 0, 0, -1},
				{"metadata", 1, 1, left, 0, 0, -1},
				{"hunk header", 2, 2, right, 0, 0, -1},
				{"context left", 3, 3, left, 0, 0, 0},
				{"context right", 3, 3, right, 0, 0, 0},
				{"deletion", 4, 4, left, 0, 0, 1},
				{"addition", 6, 4, right, 0, 0, 3},
				{"unpaired deletion", 5, 5, left, 0, 0, 2},
				{"following context", 7, 6, right, 0, 0, 4},
				{"next hunk", 9, 8, left, 0, 1, 0},
				{"next file", 12, 11, right, 1, 0, 0},
				{"after rows", 13, 12, left, 0, 0, -1},
			} {
				t.Run(tc.name, func(t *testing.T) {
					row := tc.unifiedRow
					if layout.split {
						row = tc.splitRow
					}
					cursor := diffCursor{row: row, pane: tc.pane}
					line, lineOK := v.line(cursor)
					file, fileOK := v.file(cursor)
					valid := tc.line >= 0
					testutil.RequireEqual(t, lineOK, valid, "line validity")
					testutil.RequireEqual(t, fileOK, valid, "file validity")
					if !valid {
						testutil.CheckEqual(t, line, patch.Line{},
							"invalid line")
						return
					}
					wantFile := p.Files[tc.file]
					wantLine := wantFile.Hunks[tc.hunk].Lines[tc.line]
					testutil.CheckEqual(t, line, wantLine, "source line")
					testutil.CheckEqual(t, file.DisplayPath,
						wantFile.DisplayPath, "source file")
				})
			}
		})
	}
}

func TestSourceLookupRejectsEmptyPanesAndViews(t *testing.T) {
	v := newSideBySideView(testPatch(), true)
	for _, cursor := range []diffCursor{
		{row: 4, pane: right}, {row: 8, pane: left},
	} {
		_, ok := v.line(cursor)
		testutil.CheckEqual(t, ok, false, "empty pane line: %#v", cursor)
		_, ok = v.file(cursor)
		testutil.CheckEqual(t, ok, false, "empty pane file: %#v", cursor)
	}
	for _, split := range []bool{false, true} {
		v := newDiffView(patch.Patch{}, Options{SideBySide: split})
		_, ok := v.line(diffCursor{})
		testutil.CheckEqual(t, ok, false, "empty view line")
		_, ok = v.file(diffCursor{})
		testutil.CheckEqual(t, ok, false, "empty view file")
	}
}
