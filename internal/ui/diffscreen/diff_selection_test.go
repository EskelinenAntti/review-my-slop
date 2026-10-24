package diffscreen

import (
	"slices"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestSelectionContains(t *testing.T) {
	for _, tc := range []struct {
		name      string
		selection *diffSelection
		want      []diffCursor
	}{
		{name: "no selection"},
		{
			name: "single line",
			selection: &diffSelection{
				First: diffCursor{2, left}, Last: diffCursor{2, left},
			},
			want: []diffCursor{{2, left}},
		},
		{
			name: "forward range",
			selection: &diffSelection{
				First: diffCursor{1, right}, Last: diffCursor{3, right},
			},
			want: []diffCursor{{1, right}, {2, right}, {3, right}},
		},
		{
			name: "reversed range",
			selection: &diffSelection{
				First: diffCursor{3, left}, Last: diffCursor{1, left},
			},
			want: []diffCursor{{1, left}, {2, left}, {3, left}},
		},
		{
			name: "both panes on one row",
			selection: &diffSelection{
				First: diffCursor{2, left}, Last: diffCursor{2, right},
			},
			want: []diffCursor{{2, left}, {2, right}},
		},
		{
			name: "reversed panes on one row",
			selection: &diffSelection{
				First: diffCursor{2, right}, Last: diffCursor{2, left},
			},
			want: []diffCursor{{2, left}, {2, right}},
		},
		{
			name: "cross-pane range highlights first pane",
			selection: &diffSelection{
				First: diffCursor{1, left}, Last: diffCursor{3, right},
			},
			want: []diffCursor{{1, left}, {2, left}, {3, left}},
		},
		{
			name: "reversed cross-pane range highlights first pane",
			selection: &diffSelection{
				First: diffCursor{3, right}, Last: diffCursor{1, left},
			},
			want: []diffCursor{{1, right}, {2, right}, {3, right}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for row := 0; row <= 4; row++ {
				for _, pane := range []diffPane{left, right} {
					cursor := diffCursor{row: row, pane: pane}
					testutil.RequireEqual(t,
						tc.selection.contains(cursor),
						slices.Contains(tc.want, cursor),
						"cursor=%#v", cursor,
					)
				}
			}
		})
	}
}

func TestSelectionBetween(t *testing.T) {
	for _, layout := range []struct {
		name  string
		split bool
	}{{"unified", false}, {"split", true}} {
		t.Run(layout.name, func(t *testing.T) {
			p := testPatch()
			p.Files[0].Metadata = []string{"mode changed"}
			p.Files[0].Hunks = append(p.Files[0].Hunks, patch.Hunk{
				Header: "@@ -10 +10 @@",
				Lines: []patch.Line{{
					Kind: patch.Context, Text: "next hunk",
					OldNumber: 10, NewNumber: 10,
				}},
			})
			v := newDiffView(p, Options{SideBySide: layout.split, Dark: true})
			first := mustFirst(t, v)
			added, ok := v.search("added one", first, Forward)
			testutil.RequireEqual(t, ok, true, "%s", "addition not found")
			removed, ok := v.search("removed one", first, Forward)
			testutil.RequireEqual(t, ok, true, "%s", "deletion not found")
			nextHunk, ok := v.search("next hunk", first, Forward)
			testutil.RequireEqual(t, ok, true, "%s", "next hunk not found")
			nextFile, ok := v.jumpFile(first, Forward)
			testutil.RequireEqual(t, ok, true, "%s", "next file not found")

			for _, tc := range []struct {
				name        string
				first, last diffCursor
				valid       bool
			}{
				{"single line", first, first, true},
				{"forward", first, added, true},
				{"reversed", added, first, true},
				{"change pair", removed, added, true},
				{"reversed change pair", added, removed, true},
				{
					"both panes", first,
					diffCursor{row: first.row, pane: left}, true,
				},
				{"different hunk", first, nextHunk, false},
				{"different file", first, nextFile, false},
				{"file header", first, diffCursor{row: 0, pane: right}, false},
				{"metadata", first, diffCursor{row: 1, pane: right}, false},
				{"hunk header", first, diffCursor{row: 2, pane: right}, false},
				{"negative row", diffCursor{row: -1}, first, false},
				{"past end", first, diffCursor{row: len(v.rows)}, false},
			} {
				t.Run(tc.name, func(t *testing.T) {
					selection, ok := v.selectionBetween(tc.first, tc.last)
					testutil.RequireEqual(t, ok, tc.valid,
						"selection=%#v", selection,
					)
					if ok {
						testutil.RequireEqual(t, selection.First, tc.first,
							"first endpoint=%#v", selection.First,
						)
						testutil.RequireEqual(t, selection.Last, tc.last,
							"last endpoint=%#v", selection.Last,
						)
					}
				})
			}
		})
	}
}

func TestSelectionBetweenRejectsEmptyPane(t *testing.T) {
	v := newSideBySideView(testPatch(), true)
	first := mustFirst(t, v)
	removed, ok := v.search("removed two", first, Forward)
	testutil.RequireEqual(t, ok, true, "%s", "unpaired deletion not found")
	empty := diffCursor{row: removed.row, pane: right}
	_, ok = v.selectionBetween(first, empty)
	testutil.RequireEqual(t, ok, false, "%s", "selected an empty pane")
	_, ok = v.selectionBetween(empty, first)
	testutil.RequireEqual(t, ok, false, "%s", "started in an empty pane")
}

func TestSelectionBetweenEmptyView(t *testing.T) {
	v := newUnifiedView(patch.Patch{}, true)
	_, ok := v.selectionBetween(diffCursor{}, diffCursor{})
	testutil.RequireEqual(t, ok, false, "%s", "selected code in an empty view")
}
