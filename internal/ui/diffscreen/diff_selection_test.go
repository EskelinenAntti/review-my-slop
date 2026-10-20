package diffscreen

import (
	"slices"
	"testing"
)

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
