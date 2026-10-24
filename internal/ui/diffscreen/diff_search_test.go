package diffscreen

import (
	"fmt"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestCursorNearMetadata(t *testing.T) {
	rows := []entry{
		{kind: lineRow, file: 0, leftLine: 0, rightLine: 0},
		{kind: fileRow, file: 1},
		{kind: lineRow, file: 1, leftLine: 0, rightLine: -1},
		{kind: metadataRow, file: 1},
		{kind: lineRow, file: 1, leftLine: 1, rightLine: 1},
		{kind: metadataRow, file: 1},
		{kind: lineRow, file: 1, leftLine: -1, rightLine: 2},
		{kind: fileRow, file: 2},
		{kind: fileRow, file: 3},
		{kind: lineRow, file: 4, leftLine: 0, rightLine: 0},
		{kind: fileRow, file: 5},
		{kind: metadataRow, file: 5},
		{kind: metadataRow, file: 5},
		{kind: hunkRow, file: 5},
		{kind: lineRow, file: 5, leftLine: -1, rightLine: 0},
		{kind: fileRow, file: 6},
		{kind: metadataRow, file: 6},
	}
	for _, tc := range []struct {
		name      string
		split     bool
		origin    int
		pane      diffPane
		direction Direction
		want      diffCursor
		ok        bool
	}{
		{"forward tie", true, 3, right, Forward,
			diffCursor{row: 4, pane: right}, true},
		{"backward tie", true, 3, left, Backward,
			diffCursor{row: 2, pane: left}, true},
		{"preferred pane", true, 3, left, Forward,
			diffCursor{row: 4, pane: left}, true},
		{"nearest other pane", true, 3, right, Backward,
			diffCursor{row: 2, pane: left}, true},
		{"skip previous file", true, 1, right, Backward,
			diffCursor{row: 2, pane: left}, true},
		{"right-only fallback", true, 5, left, Forward,
			diffCursor{row: 6, pane: right}, true},
		{"unified skips empty pane", false, 3, right, Backward,
			diffCursor{row: 4, pane: right}, true},
		{"unified retains pane", false, 3, left, Backward,
			diffCursor{row: 4, pane: left}, true},
		{"no code forward", true, 7, right, Forward, diffCursor{}, false},
		{"no code backward", true, 8, left, Backward, diffCursor{}, false},
		{"one boundary exhausted", true, 10, right, Backward,
			diffCursor{row: 14, pane: right}, true},
		{"trailing metadata-only file", true, 16, right, Forward,
			diffCursor{}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := &diffView{rows: rows, split: tc.split}
			cursor, ok := v.cursorNearMetadata(tc.origin, tc.pane, tc.direction)
			testutil.RequireEqual(t, ok, tc.ok, "got ok=%t, want %t", ok, tc.ok)
			testutil.RequireEqual(t, cursor, tc.want,
				"got cursor=%+v, want %+v", cursor, tc.want,
			)
		})
	}
}

func BenchmarkCursorNearMetadataWithoutCode(b *testing.B) {
	for _, unrelatedRows := range []int{10, 1000, 10000} {
		name := fmt.Sprintf("unrelated_rows_%d", unrelatedRows)
		b.Run(name, func(b *testing.B) {
			rows := make([]entry, unrelatedRows+2)
			rows[0] = entry{kind: fileRow, file: 0}
			rows[1] = entry{kind: metadataRow, file: 0}
			for index := 2; index < len(rows); index++ {
				rows[index] = entry{
					kind: lineRow, file: 1, leftLine: 0, rightLine: 0,
				}
			}
			v := &diffView{rows: rows, split: true}
			for b.Loop() {
				if _, ok := v.cursorNearMetadata(0, right, Forward); ok {
					b.Fatal("metadata-only file unexpectedly focused code")
				}
			}
		})
	}
}

func TestSearchProjectionPreservesTraversalAndMetadataFocus(t *testing.T) {
	p := patch.Patch{Files: []patch.File{
		{
			DisplayPath: "first.go",
			Metadata:    []string{"col\x1b[31mored metadata\x1b[0m"},
			Hunks: []patch.Hunk{{Header: "@@ section @@", Lines: []patch.Line{
				{Kind: patch.Context, Text: "before"},
				{Kind: patch.Deletion, Text: "needle old"},
				{Kind: patch.Addition, Text: "needle new"},
				{Kind: patch.Context, Text: "after"},
			}}},
		},
		{DisplayPath: "empty.go", Metadata: []string{"no code here"}},
		{
			DisplayPath: "last.go",
			Hunks: []patch.Hunk{{Lines: []patch.Line{
				{Kind: patch.Context, Text: "last file"},
			}}},
		},
	}}
	for _, tc := range []struct {
		name      string
		split     bool
		pane      diffPane
		direction Direction
		query     string
		want      string
	}{
		{"unified forward", false, right, Forward, "NEEDLE", "needle old"},
		{"unified backward", false, right, Backward, "needle", "needle new"},
		{"split old pane", true, left, Forward, "needle", "needle old"},
		{"split new pane", true, right, Forward, "needle", "needle new"},
		{"split other pane", true, left, Forward, "needle new", "needle new"},
		{"file header", true, right, Forward, "first.go", "before"},
		{"hunk header", true, right, Backward, "section", "before"},
		{"colored metadata", false, right, Forward, "colored", "before"},
		{"skip metadata without code", true, right, Forward, "empty.go", ""},
		{"exclude starting row", true, right, Forward, "before", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := newDiffView(p, Options{SideBySide: tc.split})
			from := mustFirst(t, v)
			from.pane = tc.pane
			cursor, ok := v.search(tc.query, from, tc.direction)
			testutil.RequireEqual(t, ok, tc.want != "", "query=%q", tc.query)
			if ok {
				line, valid := v.line(cursor)
				testutil.RequireEqual(t, valid, true,
					"search did not focus code",
				)
				testutil.RequireEqual(t, line.Text, tc.want,
					"query=%q", tc.query,
				)
			}
		})
	}
}
