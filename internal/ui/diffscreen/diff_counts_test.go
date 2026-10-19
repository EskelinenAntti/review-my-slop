package diffscreen

import (
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestPatchCountsAreIndependentOfLayout(t *testing.T) {
	for _, tc := range []struct {
		name  string
		patch patch.Patch
		want  string
	}{
		{"empty", patch.Patch{}, "+0-0"},
		{"metadata only", patch.Patch{Files: []patch.File{
			{Metadata: []string{"Binary files differ"}},
		}}, "+0-0"},
		{"multiple files and hunks", patch.Patch{Files: []patch.File{
			{Hunks: []patch.Hunk{
				{Lines: []patch.Line{
					{Kind: patch.Deletion},
					{Kind: patch.Addition},
					{Kind: patch.Addition},
				}},
				{Lines: []patch.Line{
					{Kind: patch.Context},
					{Kind: patch.LineKind(255)},
					{Kind: patch.Deletion},
				}},
			}},
			{Hunks: []patch.Hunk{{Lines: []patch.Line{
				{Kind: patch.Addition},
				{Kind: patch.Deletion},
				{Kind: patch.Deletion},
			}}}},
		}}, "+3-4"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, split := range []bool{false, true} {
				view := newDiffView(tc.patch, Options{SideBySide: split})
				testutil.RequireEqual(t, view.countsLabel, tc.want,
					"split=%v: counts = %q, want %q",
					split, view.countsLabel, tc.want,
				)
			}
		})
	}
}
