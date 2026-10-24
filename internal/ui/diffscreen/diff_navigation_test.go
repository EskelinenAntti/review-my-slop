package diffscreen

import (
	"slices"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestCursorsAt(t *testing.T) {
	rows := []entry{
		{kind: fileRow},
		{kind: lineRow, leftLine: 0, rightLine: 0},
		{kind: lineRow, leftLine: 1, rightLine: -1},
		{kind: lineRow, leftLine: -1, rightLine: 1},
	}
	for _, tc := range []struct {
		name      string
		split     bool
		row       int
		preferred diffPane
		want      []diffCursor
	}{
		{"before rows", true, -1, left, nil},
		{"after rows", true, len(rows), right, nil},
		{"metadata", true, 0, right, nil},
		{"split prefers left", true, 1, left,
			[]diffCursor{{row: 1, pane: left}, {row: 1, pane: right}}},
		{"split prefers right", true, 1, right,
			[]diffCursor{{row: 1, pane: right}, {row: 1, pane: left}}},
		{"split falls back to left", true, 2, right,
			[]diffCursor{{row: 2, pane: left}}},
		{"split falls back to right", true, 3, left,
			[]diffCursor{{row: 3, pane: right}}},
		{"unified retains left", false, 1, left,
			[]diffCursor{{row: 1, pane: left}}},
		{"unified retains right", false, 1, right,
			[]diffCursor{{row: 1, pane: right}}},
		{"unified ignores left-only cells", false, 2, left, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := &diffView{rows: rows, split: tc.split}
			got := slices.Collect(v.cursorsAt(tc.row, tc.preferred))
			testutil.CheckEqual(t, slices.Equal(got, tc.want), true,
				"selectable cursors = %v, want %v", got, tc.want)
		})
	}
}

func TestCursorsAtStopsWhenConsumerBreaks(t *testing.T) {
	v := &diffView{
		split: true,
		rows:  []entry{{kind: lineRow, leftLine: 0, rightLine: 0}},
	}
	for _, preferred := range []diffPane{left, right} {
		visited := 0
		for cursor := range v.cursorsAt(0, preferred) {
			visited++
			testutil.CheckEqual(t, cursor, diffCursor{pane: preferred},
				"first cursor")
			break
		}
		testutil.CheckEqual(t, visited, 1, "visited cursors")
	}
}
