package diffscreen

import "testing"

func TestNearestCursor(t *testing.T) {
	tests := []struct {
		name        string
		direction   direction
		maxDistance int
		rows        []int
		want        int
		found       bool
	}{
		{"target first", forward, 3, []int{4, 5, 6}, 5, true},
		{"forward tie", forward, 3, []int{4, 6}, 6, true},
		{"backward tie", backward, 3, []int{4, 6}, 4, true},
		{"closer opposite direction", forward, 3, []int{4, 7}, 4, true},
		{"inclusive distance limit", forward, 3, []int{8}, 8, true},
		{"outside distance limit", forward, 3, []int{9}, 0, false},
		{"zero distance", backward, 0, []int{4, 6}, 0, false},
		{"no eligible rows", backward, 3, nil, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cursor, ok := nearestCursor(5, tt.maxDistance, tt.direction,
				func(row int) (diffCursor, bool) {
					for _, eligible := range tt.rows {
						if row == eligible {
							return diffCursor{row: row, pane: right}, true
						}
					}
					return diffCursor{}, false
				})
			want := diffCursor{row: tt.want, pane: right}
			if ok != tt.found || ok && cursor != want {
				t.Fatalf("nearestCursor = %v, %v; want row %d, %v",
					cursor, ok, tt.want, tt.found)
			}
		})
	}
}

func TestNearestCursorCallerBoundaries(t *testing.T) {
	v := &diffView{split: true, rows: []entry{
		{kind: lineRow, file: 0, leftLine: 0, rightLine: 0},
		{kind: fileRow, file: 1, leftLine: -1, rightLine: -1},
		{kind: metadataRow, file: 1, leftLine: -1, rightLine: -1},
		{kind: lineRow, file: 1, leftLine: 0, rightLine: -1},
		{kind: lineRow, file: 1, leftLine: 1, rightLine: 1},
	}}
	t.Run("metadata falls back to other pane", func(t *testing.T) {
		cursor, ok := v.cursorNearMetadata(2, right, backward)
		if !ok || cursor != (diffCursor{row: 3, pane: left}) {
			t.Fatalf("cursorNearMetadata = %v, %v; want row 3, left",
				cursor, ok)
		}
	})
	t.Run("metadata stays in file", func(t *testing.T) {
		withoutCode := &diffView{split: true, rows: v.rows[:3]}
		cursor, ok := withoutCode.cursorNearMetadata(2, right, backward)
		if ok {
			t.Fatalf("cursorNearMetadata = %v; want no code in file", cursor)
		}
	})
	t.Run("viewport keeps requested pane", func(t *testing.T) {
		viewport := diffViewport{top: 2, height: 4}
		cursor, ok := v.nearest(2, right, backward, viewport)
		if !ok || cursor != (diffCursor{row: 4, pane: right}) {
			t.Fatalf("nearest = %v, %v; want row 4, right", cursor, ok)
		}
	})
	t.Run("viewport excludes code outside visible rows", func(t *testing.T) {
		viewport := diffViewport{top: 1, height: 3}
		if cursor, ok := v.nearest(2, right, backward, viewport); ok {
			t.Fatalf("nearest = %v; want no visible code in right pane", cursor)
		}
	})
}
