package diffscreen

import (
	"slices"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

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
