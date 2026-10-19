package diffscreen

import (
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

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
