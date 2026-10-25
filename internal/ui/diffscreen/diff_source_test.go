package diffscreen

import (
	"slices"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestEntryLineIndex(t *testing.T) {
	for _, tc := range []struct {
		name        string
		row         entry
		left, right int
	}{
		{"file header", entry{kind: fileRow}, -1, -1},
		{"metadata", entry{kind: metadataRow}, -1, -1},
		{"hunk header", entry{kind: hunkRow}, -1, -1},
		{"unified", entry{kind: lineRow, leftLine: 3, rightLine: 3}, 3, 3},
		{"paired", entry{kind: lineRow, leftLine: 0, rightLine: 2}, 0, 2},
		{"deletion", entry{kind: lineRow, leftLine: 1, rightLine: -1}, 1, -1},
		{"addition", entry{kind: lineRow, leftLine: -1, rightLine: 0}, -1, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			testutil.CheckEqual(t, tc.row.lineIndex(left), tc.left,
				"left source index")
			testutil.CheckEqual(t, tc.row.lineIndex(right), tc.right,
				"right source index")
		})
	}
}

func TestUnifiedSourceLookupIsPaneIndependent(t *testing.T) {
	v := newUnifiedView(testPatch(), true)
	for y, row := range v.rows {
		if row.kind != lineRow {
			continue
		}
		first := diffCursor{row: y, pane: left}
		last := diffCursor{row: y, pane: right}
		leftLine, leftOK := v.line(first)
		rightLine, rightOK := v.line(last)
		testutil.RequireEqual(t, leftOK && rightOK, true,
			"row %d is selectable in both panes", y)
		testutil.CheckEqual(t, leftLine, rightLine,
			"row %d identifies the same source line", y)
		selection, ok := v.selectionBetween(first, last)
		testutil.RequireEqual(t, ok, true,
			"row %d permits cross-pane selection", y)
		want := []patch.Line{leftLine}
		testutil.CheckEqual(t, slices.Equal(v.lines(selection), want), true,
			"row %d does not duplicate the selected source line", y)
	}
}

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
