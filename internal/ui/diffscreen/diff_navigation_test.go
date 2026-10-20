package diffscreen

import (
	"slices"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestRowCursors(t *testing.T) {
	for _, tc := range []struct {
		name      string
		split     bool
		row       entry
		y         int
		preferred diffPane
		want      []diffCursor
	}{
		{
			"left preferred", true, entry{kind: lineRow}, 0, left,
			[]diffCursor{{0, left}, {0, right}},
		},
		{
			"right preferred", true, entry{kind: lineRow}, 0, right,
			[]diffCursor{{0, right}, {0, left}},
		},
		{
			"empty preferred pane", true,
			entry{kind: lineRow, leftLine: -1}, 0, left,
			[]diffCursor{{0, right}},
		},
		{
			"empty fallback pane", true,
			entry{kind: lineRow, rightLine: -1}, 0, left,
			[]diffCursor{{0, left}},
		},
		{
			"unified retains left identity", false,
			entry{kind: lineRow, leftLine: -1}, 0, left,
			[]diffCursor{{0, left}},
		},
		{
			"unified retains right identity", false,
			entry{kind: lineRow}, 0, right, []diffCursor{{0, right}},
		},
		{"metadata", true, entry{kind: metadataRow}, 0, left, nil},
		{"negative row", true, entry{kind: lineRow}, -1, left, nil},
		{"past last row", true, entry{kind: lineRow}, 1, right, nil},
		{
			"empty row", true,
			entry{kind: lineRow, leftLine: -1, rightLine: -1}, 0, left, nil,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := &diffView{split: tc.split, rows: []entry{tc.row}}
			got := slices.Collect(v.rowCursors(tc.y, tc.preferred))
			if !slices.Equal(got, tc.want) {
				t.Fatalf("row cursors = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestRowCursorsStopsWhenConsumerBreaks(t *testing.T) {
	v := &diffView{split: true, rows: []entry{{kind: lineRow}}}
	var visited []diffCursor
	for cursor := range v.rowCursors(0, right) {
		visited = append(visited, cursor)
		break
	}
	if !slices.Equal(visited, []diffCursor{{0, right}}) {
		t.Fatalf("visited cursors = %v, want only the preferred pane", visited)
	}
}

func TestJumpFile(t *testing.T) {
	for _, tc := range []struct {
		name      string
		split     bool
		start     diffCursor
		direction direction
		want      diffCursor
		ok        bool
	}{
		{
			"forward left", true, diffCursor{1, left},
			forward, diffCursor{6, left}, true,
		},
		{
			"forward right", true, diffCursor{1, right},
			forward, diffCursor{8, right}, true,
		},
		{
			"backward left", true, diffCursor{11, left},
			backward, diffCursor{6, left}, true,
		},
		{
			"backward right", true, diffCursor{11, right},
			backward, diffCursor{8, right}, true,
		},
		{
			"unified forward", false, diffCursor{1, right},
			forward, diffCursor{8, right}, true,
		},
		{
			"unified backward", false, diffCursor{11, right},
			backward, diffCursor{8, right}, true,
		},
		{
			"first file", true, diffCursor{2, right},
			backward, diffCursor{}, false,
		},
		{
			"last file", true, diffCursor{10, left},
			forward, diffCursor{}, false,
		},
		{
			"invalid row", true, diffCursor{-1, right},
			forward, diffCursor{}, false,
		},
		{
			"metadata row", true, diffCursor{4, right},
			forward, diffCursor{}, false,
		},
		{
			"empty pane", true, diffCursor{6, right},
			forward, diffCursor{}, false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := &diffView{split: tc.split, rows: []entry{
				{kind: fileRow, file: 0},
				{kind: lineRow, file: 0, leftLine: 0, rightLine: 0},
				{kind: lineRow, file: 0, leftLine: 1, rightLine: 1},
				{kind: fileRow, file: 1},
				{kind: metadataRow, file: 1},
				{kind: fileRow, file: 2},
				{kind: lineRow, file: 2, leftLine: 0, rightLine: -1},
				{kind: fileRow, file: 3},
				{kind: lineRow, file: 3, leftLine: -1, rightLine: 0},
				{kind: fileRow, file: 4},
				{kind: lineRow, file: 4, leftLine: 0, rightLine: 0},
				{kind: lineRow, file: 4, leftLine: 1, rightLine: 1},
			}}
			got, ok := v.jumpFile(tc.start, tc.direction)
			testutil.RequireEqual(t, ok, tc.ok, "jump available")
			testutil.RequireEqual(t, got, tc.want, "jump cursor")
		})
	}
}
