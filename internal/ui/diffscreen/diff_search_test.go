package diffscreen

import (
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestSearchMetadataMatchesVisibleTextOnly(t *testing.T) {
	for _, split := range []bool{false, true} {
		for _, tc := range []struct {
			name, query string
			want        bool
		}{
			{name: "case insensitive", query: "needle", want: true},
			{name: "missing", query: "missing"},
			{name: "ignores ANSI escapes", query: "31m"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				p := patchWithLines([]patch.Line{
					{Kind: patch.Context, Text: "code"},
				})
				p.Files[0].Metadata = []string{"\x1b[31mNEEDLE\x1b[0m"}
				v := newDiffView(p, Options{SideBySide: split})
				assertMetadataSearch(t, v, tc.query, tc.want)
			})
		}
	}
}

func assertMetadataSearch(t *testing.T, v *diffView, query string, want bool) {
	t.Helper()
	for _, direction := range []Direction{Forward, Backward} {
		cursor, ok := v.searchRow(query, 1, right, direction)
		testutil.RequireEqual(t, ok, want, "metadata match")
		if ok {
			line, valid := v.line(cursor)
			testutil.RequireEqual(t, valid, true, "match has no code location")
			testutil.RequireEqual(t, line.Text, "code", "wrong code line")
		}
	}
}
