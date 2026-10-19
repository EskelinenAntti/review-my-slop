package diffscreen

import (
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestChangeCountsAreIndependentOfLayout(t *testing.T) {
	for _, tc := range []struct {
		name  string
		kinds []patch.LineKind
		want  string
	}{
		{"empty", nil, "+0-0"},
		{"context", []patch.LineKind{patch.Context}, "+0-0"},
		{"additions", []patch.LineKind{patch.Addition, patch.Addition}, "+2-0"},
		{"deletions", []patch.LineKind{patch.Deletion, patch.Deletion}, "+0-2"},
		{"paired", []patch.LineKind{patch.Deletion, patch.Addition}, "+1-1"},
		{
			"more additions",
			[]patch.LineKind{patch.Deletion, patch.Addition, patch.Addition},
			"+2-1",
		},
		{
			"more deletions",
			[]patch.LineKind{patch.Deletion, patch.Deletion, patch.Addition},
			"+1-2",
		},
		{
			"separate blocks",
			[]patch.LineKind{patch.Addition, patch.Context, patch.Deletion},
			"+1-1",
		},
		{"unknown kind", []patch.LineKind{patch.LineKind(255)}, "+0-0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			lines := make([]patch.Line, len(tc.kinds))
			for index, kind := range tc.kinds {
				lines[index] = patch.Line{Kind: kind, Text: "code"}
			}
			assertChangeCounts(t, patchWithLines(lines), tc.want)
		})
	}
}

func TestChangeCountsIncludeEveryFileAndHunk(t *testing.T) {
	p := patch.Patch{Files: []patch.File{
		{Hunks: []patch.Hunk{
			{Lines: []patch.Line{{Kind: patch.Addition}}},
			{Lines: []patch.Line{{Kind: patch.Deletion}}},
		}},
		{Hunks: []patch.Hunk{
			{Lines: []patch.Line{
				{Kind: patch.Addition}, {Kind: patch.Context},
			}},
		}},
	}}
	assertChangeCounts(t, p, "+2-1")
}

func assertChangeCounts(t *testing.T, p patch.Patch, want string) {
	t.Helper()
	for _, split := range []bool{false, true} {
		v := newDiffView(p, Options{SideBySide: split})
		testutil.RequireEqual(t, v.countsLabel, want,
			"change counts differ with side-by-side=%t", split)
	}
}
