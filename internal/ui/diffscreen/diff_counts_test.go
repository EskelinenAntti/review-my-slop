package diffscreen

import (
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestChangeCountsDoNotDependOnLayout(t *testing.T) {
	for _, tc := range []struct {
		name  string
		patch patch.Patch
		want  string
	}{
		{name: "empty", want: "+0-0"},
		{
			name: "metadata only",
			patch: patch.Patch{Files: []patch.File{{
				DisplayPath: "binary", Metadata: []string{"binary file"},
			}}},
			want: "+0-0",
		},
		{
			name: "multiple files and hunks with unequal change blocks",
			patch: patch.Patch{Files: []patch.File{
				{DisplayPath: "first", Hunks: []patch.Hunk{
					{Lines: []patch.Line{
						{Kind: patch.Deletion},
						{Kind: patch.Addition},
						{Kind: patch.Addition},
						{Kind: patch.Context},
						{Kind: patch.LineKind(255)},
					}},
					{Lines: []patch.Line{{Kind: patch.Deletion}}},
				}},
				{DisplayPath: "second", Hunks: []patch.Hunk{
					{Lines: []patch.Line{{Kind: patch.Addition}}},
				}},
			}},
			want: "+3-2",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, split := range []bool{false, true} {
				v := newDiffView(tc.patch, Options{SideBySide: split})
				testutil.RequireEqual(t, v.countsLabel, tc.want,
					"split=%t: counts=%q, want %q",
					split, v.countsLabel, tc.want,
				)
			}
		})
	}
}
