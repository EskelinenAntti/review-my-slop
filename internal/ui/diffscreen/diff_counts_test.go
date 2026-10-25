package diffscreen

import (
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestChangeCountsAcrossLayouts(t *testing.T) {
	tests := []struct {
		name  string
		files [][][]patch.LineKind
		want  string
	}{
		{name: "empty patch", want: "+0-0"},
		{name: "metadata only", files: [][][]patch.LineKind{{}}, want: "+0-0"},
		{
			name:  "context only",
			files: [][][]patch.LineKind{{{patch.Context, patch.Context}}},
			want:  "+0-0",
		},
		{
			name: "paired and unpaired changes across files and hunks",
			files: [][][]patch.LineKind{
				{
					{
						patch.Deletion, patch.Deletion,
						patch.Addition, patch.Context,
					},
					{patch.Addition, patch.Addition},
				},
				{
					{patch.Deletion},
					{patch.Deletion, patch.Addition, patch.Addition},
				},
			},
			want: "+5-4",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var p patch.Patch
			for _, hunks := range tt.files {
				var file patch.File
				for _, kinds := range hunks {
					var hunk patch.Hunk
					for _, kind := range kinds {
						hunk.Lines = append(hunk.Lines, patch.Line{Kind: kind})
					}
					file.Hunks = append(file.Hunks, hunk)
				}
				p.Files = append(p.Files, file)
			}
			for _, split := range []bool{false, true} {
				v := newDiffView(p, Options{SideBySide: split})
				testutil.RequireEqual(t, v.countsLabel, tt.want,
					"split=%v: counts = %q", split, v.countsLabel,
				)
			}
		})
	}
}
