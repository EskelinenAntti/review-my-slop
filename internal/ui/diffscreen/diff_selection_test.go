package diffscreen

import "testing"

func TestTranslateSelection(t *testing.T) {
	v := &diffView{
		split: true,
		rows: []entry{
			{kind: lineRow, file: 0, hunk: 0},
			{kind: lineRow, file: 0, hunk: 0},
			{kind: lineRow, file: 0, hunk: 1},
			{kind: lineRow, file: 1, hunk: 0},
			{kind: metadataRow},
		},
	}
	for _, tt := range []struct {
		name            string
		first, last     diffCursor
		firstOK, lastOK bool
		valid           bool
	}{
		{"forward", diffCursor{0, left}, diffCursor{1, left}, true, true, true},
		{"backward", diffCursor{1, left}, diffCursor{0, left},
			true, true, true},
		{"across panes", diffCursor{0, left}, diffCursor{0, right},
			true, true, true},
		{"missing first", diffCursor{}, diffCursor{1, left},
			false, true, false},
		{"missing last", diffCursor{0, left}, diffCursor{}, true, false, false},
		{"different hunk", diffCursor{0, left}, diffCursor{2, left},
			true, true, false},
		{"different file", diffCursor{0, left}, diffCursor{3, left},
			true, true, false},
		{"metadata", diffCursor{0, left}, diffCursor{4, left},
			true, true, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			original := diffSelection{
				First: diffCursor{0, right}, Last: diffCursor{1, right},
			}
			selection, ok := v.translateSelection(original,
				func(cursor diffCursor) (diffCursor, bool) {
					switch cursor {
					case original.First:
						return tt.first, tt.firstOK
					case original.Last:
						return tt.last, tt.lastOK
					default:
						t.Fatalf("unexpected endpoint: %#v", cursor)
						return diffCursor{}, false
					}
				})
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

func TestSelectionBetween(t *testing.T) {
	v := &diffView{
		split: true,
		rows: []entry{
			{kind: lineRow, file: 0, hunk: 0,
				panes: [2]paneContent{{line: 0}, {line: 0}}},
			{kind: lineRow, file: 0, hunk: 0,
				panes: [2]paneContent{{line: 1}, {line: 1}}},
			{kind: lineRow, file: 0, hunk: 1,
				panes: [2]paneContent{{line: 0}, {line: 0}}},
			{kind: lineRow, file: 1, hunk: 0,
				panes: [2]paneContent{{line: 0}, {line: 0}}},
			{kind: hunkRow, file: 0, hunk: 0},
			{kind: lineRow, file: 0, hunk: 0,
				panes: [2]paneContent{{line: -1}, {line: 2}}},
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
