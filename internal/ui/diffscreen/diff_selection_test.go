package diffscreen

import (
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

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
