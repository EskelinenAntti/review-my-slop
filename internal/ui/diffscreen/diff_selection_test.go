package diffscreen

import (
	"slices"
	"testing"
)

func TestSelectionBetween(t *testing.T) {
	v := &diffView{
		split: true,
		rows: []entry{
			{kind: fileRow, file: 0, hunk: -1},
			{kind: lineRow, file: 0, hunk: 0, leftLine: 0, rightLine: 1},
			{kind: lineRow, file: 0, hunk: 0, leftLine: 2, rightLine: -1},
			{kind: lineRow, file: 0, hunk: 1, leftLine: 0, rightLine: 0},
			{kind: lineRow, file: 1, hunk: 0, leftLine: 0, rightLine: 0},
		},
	}
	tests := []struct {
		name        string
		first, last diffCursor
		valid       bool
	}{
		{"single line", diffCursor{1, left}, diffCursor{1, left}, true},
		{"forward range", diffCursor{1, left}, diffCursor{2, left}, true},
		{"reverse range", diffCursor{2, left}, diffCursor{1, left}, true},
		{"paired panes", diffCursor{1, left}, diffCursor{1, right}, true},
		{"mixed panes", diffCursor{2, left}, diffCursor{1, right}, true},
		{"different hunk", diffCursor{1, left}, diffCursor{3, left}, false},
		{"different file", diffCursor{1, left}, diffCursor{4, left}, false},
		{"metadata endpoint", diffCursor{1, left}, diffCursor{0, left}, false},
		{"empty pane", diffCursor{1, right}, diffCursor{2, right}, false},
		{"invalid start", diffCursor{-1, left}, diffCursor{1, left}, false},
		{"invalid end", diffCursor{1, left}, diffCursor{5, left}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			selection, ok := v.selectionBetween(tt.first, tt.last)
			if ok != tt.valid {
				t.Fatalf("selectionBetween() valid = %v, want %v",
					ok, tt.valid)
			}
			want := diffSelection{First: tt.first, Last: tt.last}
			if ok && selection != want {
				t.Errorf("selectionBetween() changed endpoints: %+v", selection)
			}
		})
	}
}

func TestSelectionContains(t *testing.T) {
	tests := []struct {
		name      string
		selection *diffSelection
		included  []diffCursor
	}{
		{
			name: "no selection",
		},
		{
			name: "single cursor",
			selection: &diffSelection{
				First: diffCursor{row: 2, pane: right},
				Last:  diffCursor{row: 2, pane: right},
			},
			included: []diffCursor{{row: 2, pane: right}},
		},
		{
			name: "forward range",
			selection: &diffSelection{
				First: diffCursor{row: 2, pane: left},
				Last:  diffCursor{row: 4, pane: left},
			},
			included: []diffCursor{
				{row: 2, pane: left},
				{row: 3, pane: left},
				{row: 4, pane: left},
			},
		},
		{
			name: "reverse range",
			selection: &diffSelection{
				First: diffCursor{row: 4, pane: right},
				Last:  diffCursor{row: 2, pane: right},
			},
			included: []diffCursor{
				{row: 2, pane: right},
				{row: 3, pane: right},
				{row: 4, pane: right},
			},
		},
		{
			name: "both panes on one row",
			selection: &diffSelection{
				First: diffCursor{row: 2, pane: left},
				Last:  diffCursor{row: 2, pane: right},
			},
			included: []diffCursor{{row: 2, pane: left}, {row: 2, pane: right}},
		},
		{
			name: "multirow range highlights the initial pane",
			selection: &diffSelection{
				First: diffCursor{row: 2, pane: left},
				Last:  diffCursor{row: 4, pane: right},
			},
			included: []diffCursor{
				{row: 2, pane: left},
				{row: 3, pane: left},
				{row: 4, pane: left},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for row := range 6 {
				for _, pane := range []diffPane{left, right} {
					cursor := diffCursor{row: row, pane: pane}
					want := slices.Contains(tt.included, cursor)
					if got := tt.selection.contains(cursor); got != want {
						t.Errorf("contains(%+v) = %v, want %v",
							cursor, got, want)
					}
				}
			}
		})
	}
}
