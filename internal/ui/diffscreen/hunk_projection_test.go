package diffscreen

import (
	"slices"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestHunkLinePairs(t *testing.T) {
	for _, tc := range []struct {
		name  string
		kinds []patch.LineKind
		want  [][2]int
	}{
		{"empty", nil, nil},
		{
			"additions",
			[]patch.LineKind{patch.Addition, patch.Addition},
			[][2]int{{-1, 0}, {-1, 1}},
		},
		{
			"deletions",
			[]patch.LineKind{patch.Deletion, patch.Deletion},
			[][2]int{{0, -1}, {1, -1}},
		},
		{
			"more deletions",
			[]patch.LineKind{patch.Deletion, patch.Deletion, patch.Addition},
			[][2]int{{0, 2}, {1, -1}},
		},
		{
			"more additions",
			[]patch.LineKind{patch.Deletion, patch.Addition, patch.Addition},
			[][2]int{{0, 1}, {-1, 2}},
		},
		{
			"context separates blocks",
			[]patch.LineKind{patch.Deletion, patch.Context, patch.Addition},
			[][2]int{{0, -1}, {1, 1}, {-1, 2}},
		},
		{
			"addition before replacement",
			[]patch.LineKind{patch.Addition, patch.Deletion, patch.Addition},
			[][2]int{{-1, 0}, {1, 2}},
		},
		{
			"consecutive replacements",
			[]patch.LineKind{
				patch.Deletion, patch.Addition, patch.Deletion, patch.Addition,
			},
			[][2]int{{0, 1}, {2, 3}},
		},
		{
			"unknown kind", []patch.LineKind{patch.LineKind(255)},
			[][2]int{{0, 0}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			lines := make([]patch.Line, len(tc.kinds))
			for index, kind := range tc.kinds {
				lines[index].Kind = kind
			}
			for _, split := range []bool{false, true} {
				pairs := unifiedHunkLinePairs(lines)
				if split {
					pairs = splitHunkLinePairs(lines)
				}
				var got [][2]int
				for leftLine, rightLine := range pairs {
					got = append(got, [2]int{leftLine, rightLine})
				}
				want := tc.want
				if !split {
					want = make([][2]int, len(lines))
					for index := range lines {
						want[index] = [2]int{index, index}
					}
				}
				testutil.RequireEqual(t, slices.Equal(got, want), true,
					"split=%v: pairs=%v, want %v", split, got, want,
				)
			}
		})
	}
}

func TestHunkLinePairsStopsWhenConsumerBreaks(t *testing.T) {
	for _, kind := range []patch.LineKind{
		patch.Context, patch.Addition, patch.Deletion,
	} {
		for _, split := range []bool{false, true} {
			lines := []patch.Line{{Kind: kind}, {Kind: kind}}
			pairs := unifiedHunkLinePairs(lines)
			if split {
				pairs = splitHunkLinePairs(lines)
			}
			count := 0
			for range pairs {
				count++
				break
			}
			testutil.RequireEqual(t, count, 1, "kind=%v, split=%v", kind, split)
		}
	}
}
