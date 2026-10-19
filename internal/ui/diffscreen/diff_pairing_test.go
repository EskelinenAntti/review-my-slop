package diffscreen

import (
	"iter"
	"slices"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestLinePairsPreserveSourceIndices(t *testing.T) {
	lines := []patch.Line{
		{Kind: patch.Context},
		{Kind: patch.Deletion},
		{Kind: patch.Deletion},
		{Kind: patch.Addition},
		{Kind: patch.Context},
		{Kind: patch.Addition},
	}
	for _, tc := range []struct {
		name  string
		pairs func([]patch.Line) iter.Seq2[int, int]
		want  [][2]int
	}{
		{
			"unified", unifiedLinePairs,
			[][2]int{{0, 0}, {1, 1}, {2, 2}, {3, 3}, {4, 4}, {5, 5}},
		},
		{
			"split", splitLinePairs,
			[][2]int{{0, 0}, {1, 3}, {2, -1}, {4, 4}, {-1, 5}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var got [][2]int
			for left, right := range tc.pairs(lines) {
				got = append(got, [2]int{left, right})
			}
			testutil.RequireEqual(t, slices.Equal(got, tc.want), true,
				"line pairs=%v, want %v", got, tc.want)
		})
	}
}

func TestLinePairsStopWhenConsumerBreaks(t *testing.T) {
	kinds := []patch.LineKind{patch.Context, patch.Deletion, patch.Addition}
	for _, kind := range kinds {
		for _, pairs := range []func([]patch.Line) iter.Seq2[int, int]{
			unifiedLinePairs, splitLinePairs,
		} {
			lines := []patch.Line{{Kind: kind}, {Kind: kind}}
			calls := 0
			pairs(lines)(func(int, int) bool {
				calls++
				return false
			})
			testutil.RequireEqual(t, calls, 1,
				"kind=%v: yield called %d times", kind, calls)
		}
	}
}
