package diffscreen

import (
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestChangeCountsDoNotDependOnProjection(t *testing.T) {
	for _, tt := range []struct {
		name  string
		hunks []patch.Hunk
		want  string
	}{
		{"empty", nil, "+0-0"},
		{"context", []patch.Hunk{{Lines: []patch.Line{
			{Kind: patch.Context},
		}}}, "+0-0"},
		{"additions", []patch.Hunk{{Lines: []patch.Line{
			{Kind: patch.Addition}, {Kind: patch.Addition},
		}}}, "+2-0"},
		{"deletions", []patch.Hunk{{Lines: []patch.Line{
			{Kind: patch.Deletion}, {Kind: patch.Deletion},
		}}}, "+0-2"},
		{"uneven change blocks", []patch.Hunk{
			{Lines: []patch.Line{
				{Kind: patch.Context},
				{Kind: patch.Deletion}, {Kind: patch.Deletion},
				{Kind: patch.Addition}, {Kind: patch.Addition},
				{Kind: patch.Addition}, {Kind: patch.Context},
			}},
			{Lines: []patch.Line{
				{Kind: patch.Deletion}, {Kind: patch.Addition},
			}},
		}, "+4-3"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			p := patch.Patch{Files: []patch.File{{
				DisplayPath: "file.go", Hunks: tt.hunks,
			}}}
			for _, split := range []bool{false, true} {
				v := newDiffView(p, Options{SideBySide: split})
				testutil.RequireEqual(t, v.countsLabel, tt.want,
					"split=%v counts = %q, want %q",
					split, v.countsLabel, tt.want,
				)
			}
		})
	}
}
