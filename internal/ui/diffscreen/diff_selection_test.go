package diffscreen

import "testing"

func TestSelectionBetween(t *testing.T) {
	v := &diffView{
		split: true,
		rows: []entry{
			{kind: lineRow, file: 0, hunk: 0, leftLine: 0, rightLine: 0},
			{kind: lineRow, file: 0, hunk: 0, leftLine: 1, rightLine: 1},
			{kind: lineRow, file: 0, hunk: 1, leftLine: 0, rightLine: 0},
			{kind: lineRow, file: 1, hunk: 0, leftLine: 0, rightLine: 0},
			{kind: hunkRow, file: 0, hunk: 0},
			{kind: lineRow, file: 0, hunk: 0, leftLine: -1, rightLine: 2},
		},
	}
	for _, tt := range []struct {
		name        string
		first, last diffCursor
		valid       bool
	}{
		{"single line", diffCursor{0, right}, diffCursor{0, right}, true},
		{"forward", diffCursor{0, right}, diffCursor{1, right}, true},
		{"backward", diffCursor{1, right}, diffCursor{0, right}, true},
		{"across panes", diffCursor{0, left}, diffCursor{1, right}, true},
		{"different hunk", diffCursor{0, right}, diffCursor{2, right}, false},
		{"different file", diffCursor{0, right}, diffCursor{3, right}, false},
		{"metadata", diffCursor{0, right}, diffCursor{4, right}, false},
		{"empty pane", diffCursor{0, right}, diffCursor{5, left}, false},
		{"invalid first", diffCursor{-1, right}, diffCursor{0, right}, false},
		{"invalid last", diffCursor{0, right}, diffCursor{6, right}, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			selection, ok := v.selectionBetween(tt.first, tt.last)
			if ok != tt.valid {
				t.Fatalf("valid = %v, want %v", ok, tt.valid)
			}
			want := diffSelection{First: tt.first, Last: tt.last}
			if ok && selection != want {
				t.Fatalf("selection = %#v, want %#v", selection, want)
			}
		})
	}
}
