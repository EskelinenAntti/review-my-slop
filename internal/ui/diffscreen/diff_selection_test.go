package diffscreen

import (
	"slices"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestSelectionBetween(t *testing.T) {
	p := testPatch()
	p.Files[0].Hunks = append(p.Files[0].Hunks, patch.Hunk{
		Header: "@@ -10 +10 @@",
		Lines:  []patch.Line{{Kind: patch.Context, Text: "later"}},
	})
	v := newSideBySideView(p, true)
	first := mustFirst(t, v)
	removed, _ := v.search("removed one", first, Forward)
	after, _ := v.search("after", first, Forward)
	later, _ := v.search("later", first, Forward)
	other, _ := v.search("other", first, Forward)
	for _, tt := range []struct {
		name        string
		first, last diffCursor
		wantOK      bool
	}{
		{"single line", first, first, true},
		{"forward range", first, after, true},
		{"backward range", after, first, true},
		{"both panes", removed, diffCursor{removed.row, right}, true},
		{"different hunk", first, later, false},
		{"different file", first, other, false},
		{"invalid start", diffCursor{-1, right}, after, false},
		{"invalid end", first, diffCursor{len(v.rows), right}, false},
		{"metadata", first, diffCursor{0, right}, false},
		{"empty pane", first, diffCursor{removed.row + 1, right}, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			selection, ok := v.selectionBetween(tt.first, tt.last)
			testutil.RequireEqual(t, ok, tt.wantOK, "selection validity")
			want := diffSelection{}
			if tt.wantOK {
				want = diffSelection{First: tt.first, Last: tt.last}
			}
			testutil.RequireEqual(t, selection, want, "selection endpoints")
		})
	}
}

func TestSwitchSelectionPane(t *testing.T) {
	p := patchWithHunks([]patch.Hunk{
		{Header: "@@", Lines: []patch.Line{
			{Kind: patch.Context, Text: "before"},
			{Kind: patch.Deletion, Text: "removed"},
			{Kind: patch.Deletion, Text: "extra removal"},
			{Kind: patch.Addition, Text: "added"},
			{Kind: patch.Context, Text: "after"},
		}},
		{Header: "@@ later @@", Lines: []patch.Line{
			{Kind: patch.Context, Text: "later"},
		}},
	})
	v := newSideBySideView(p, true)
	before := mustFirst(t, v)
	removed, _ := v.search("removed", before, Forward)
	extra, _ := v.search("extra removal", before, Forward)
	after, _ := v.search("after", before, Forward)
	later, _ := v.search("later", before, Forward)
	for _, tt := range []struct {
		name        string
		first, last diffCursor
		pane        diffPane
		want        diffSelection
		wantOK      bool
	}{
		{"forward range", before, after, left,
			diffSelection{
				diffCursor{before.row, left}, diffCursor{after.row, left},
			}, true},
		{"backward range", after, before, left,
			diffSelection{
				diffCursor{after.row, left}, diffCursor{before.row, left},
			}, true},
		{"empty pane falls back", removed, extra, right,
			diffSelection{
				diffCursor{removed.row, right}, diffCursor{removed.row, right},
			}, true},
		{"different hunks", before, later, left, diffSelection{}, false},
		{"invalid start", diffCursor{-1, left}, after, right,
			diffSelection{}, false},
		{"invalid end", before, diffCursor{len(v.rows), left}, right,
			diffSelection{}, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			selection := diffSelection{First: tt.first, Last: tt.last}
			got, ok := v.switchSelectionPane(selection, tt.pane)
			testutil.RequireEqual(t, ok, tt.wantOK, "selection validity")
			testutil.RequireEqual(t, got, tt.want, "selection endpoints")
		})
	}
	unified := newDiffView(p, Options{})
	first := mustFirst(t, unified)
	_, ok := unified.switchSelectionPane(beginSelection(first), left)
	testutil.RequireEqual(t, ok, false, "unified view has no other pane")
}

func TestSelectionContains(t *testing.T) {
	t.Run("no selection", func(t *testing.T) {
		assertSelectionContains(t, nil, nil)
	})
	for _, tt := range []struct {
		name        string
		first, last diffCursor
		want        []diffCursor
	}{
		{
			name:  "single line",
			first: diffCursor{2, right}, last: diffCursor{2, right},
			want: []diffCursor{{2, right}},
		},
		{
			name:  "forward range",
			first: diffCursor{1, left}, last: diffCursor{3, left},
			want: []diffCursor{{1, left}, {2, left}, {3, left}},
		},
		{
			name:  "backward range",
			first: diffCursor{3, right}, last: diffCursor{1, right},
			want: []diffCursor{{1, right}, {2, right}, {3, right}},
		},
		{
			name:  "both panes on one row",
			first: diffCursor{2, left}, last: diffCursor{2, right},
			want: []diffCursor{{2, left}, {2, right}},
		},
		{
			name:  "both panes on one row reversed",
			first: diffCursor{2, right}, last: diffCursor{2, left},
			want: []diffCursor{{2, left}, {2, right}},
		},
		{
			name:  "multi-row range highlights starting pane",
			first: diffCursor{1, left}, last: diffCursor{3, right},
			want: []diffCursor{{1, left}, {2, left}, {3, left}},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			selection := diffSelection{First: tt.first, Last: tt.last}
			assertSelectionContains(t, &selection, tt.want)
		})
	}
}

func TestSelectionLinesAcrossPanes(t *testing.T) {
	context := patch.Line{Kind: patch.Context, Text: "shared"}
	removed := patch.Line{Kind: patch.Deletion, Text: "removed"}
	added := patch.Line{Kind: patch.Addition, Text: "added"}
	p := patchWithHunks([]patch.Hunk{{Header: "@@", Lines: []patch.Line{
		context, removed, added, {Kind: patch.Context, Text: "after"},
	}}})
	for _, tt := range []struct {
		name  string
		split bool
		query string
		pane  diffPane
		want  []patch.Line
	}{
		{"unified context", false, "shared", left, []patch.Line{context}},
		{"unified context reversed", false, "shared", right,
			[]patch.Line{context}},
		{"split context", true, "shared", left, []patch.Line{context}},
		{"split context reversed", true, "shared", right,
			[]patch.Line{context}},
		{"unified change", false, "removed", left, []patch.Line{removed}},
		{"unified change reversed", false, "removed", right,
			[]patch.Line{removed}},
		{"split change", true, "removed", left, []patch.Line{removed, added}},
		{"split change reversed", true, "removed", right,
			[]patch.Line{added, removed}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			v := newDiffView(p, Options{SideBySide: tt.split})
			last, ok := v.last()
			testutil.RequireEqual(t, ok, true, "find last row")
			cursor, ok := v.search(tt.query, last, Forward)
			testutil.RequireEqual(t, ok, true, "find selected row")
			selection := diffSelection{
				First: diffCursor{cursor.row, tt.pane},
				Last:  diffCursor{cursor.row, tt.pane.other()},
			}
			testutil.RequireEqual(t,
				slices.Equal(v.lines(selection), tt.want), true,
				"selected lines")
		})
	}
}

func assertSelectionContains(
	t *testing.T,
	selection *diffSelection,
	want []diffCursor,
) {
	t.Helper()
	for row := 0; row <= 4; row++ {
		for _, pane := range []diffPane{left, right} {
			cursor := diffCursor{row, pane}
			testutil.RequireEqual(t,
				selection.contains(cursor), slices.Contains(want, cursor),
				"selection membership at %+v", cursor)
		}
	}
}
