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

func TestSplitLinePairsExhaustive(t *testing.T) {
	kinds := []patch.LineKind{patch.Context, patch.Deletion, patch.Addition}
	var check func([]patch.Line)
	check = func(lines []patch.Line) {
		var pairs [][2]int
		for left, right := range splitLinePairs(lines) {
			pairs = append(pairs, [2]int{left, right})
		}
		checkSplitPairIndices(t, lines, pairs, 0, patch.Addition)
		checkSplitPairIndices(t, lines, pairs, 1, patch.Deletion)
		checkSplitPairStops(t, lines, pairs)
		if len(lines) < 7 {
			for _, kind := range kinds {
				check(append(lines, patch.Line{Kind: kind}))
			}
		}
	}
	check(nil)
}

func checkSplitPairIndices(
	t *testing.T,
	lines []patch.Line,
	pairs [][2]int,
	pane int,
	excluded patch.LineKind,
) {
	t.Helper()
	var want, got []int
	for index, line := range lines {
		if line.Kind != excluded {
			want = append(want, index)
		}
	}
	for _, pair := range pairs {
		if index := pair[pane]; index >= 0 {
			got = append(got, index)
		}
	}
	testutil.RequireEqual(t, slices.Equal(got, want), true,
		"lines=%v: pane=%d indices=%v, want %v", lines, pane, got, want)
}

func checkSplitPairStops(t *testing.T, lines []patch.Line, pairs [][2]int) {
	t.Helper()
	for stop := 1; stop <= len(pairs); stop++ {
		var prefix [][2]int
		splitLinePairs(lines)(func(left, right int) bool {
			prefix = append(prefix, [2]int{left, right})
			return len(prefix) < stop
		})
		testutil.RequireEqual(t, slices.Equal(prefix, pairs[:stop]), true,
			"lines=%v: stopped pairs=%v, want %v", lines, prefix, pairs[:stop])
	}
}
