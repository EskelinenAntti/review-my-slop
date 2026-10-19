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
	removed, _ := v.search("removed one", first, forward)
	after, _ := v.search("after", first, forward)
	later, _ := v.search("later", first, forward)
	other, _ := v.search("other", first, forward)
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
