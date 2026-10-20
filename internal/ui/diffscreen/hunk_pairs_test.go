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
		split bool
		want  [][2]int
	}{
		{name: "empty"},
		{
			name: "unified preserves indices",
			kinds: []patch.LineKind{
				patch.Context, patch.Deletion, patch.Addition,
				patch.LineKind(255),
			},
			want: [][2]int{{0, 0}, {1, 1}, {2, 2}, {3, 3}},
		},
		{
			name: "split pads deletions",
			kinds: []patch.LineKind{
				patch.Deletion, patch.Deletion, patch.Addition,
			},
			split: true,
			want:  [][2]int{{0, 2}, {1, -1}},
		},
		{
			name: "split pads additions",
			kinds: []patch.LineKind{
				patch.Deletion, patch.Addition, patch.Addition,
			},
			split: true,
			want:  [][2]int{{0, 1}, {-1, 2}},
		},
		{
			name: "split separates change blocks",
			kinds: []patch.LineKind{
				patch.Addition, patch.Deletion, patch.Context,
				patch.LineKind(255), patch.Deletion, patch.Addition,
			},
			split: true,
			want:  [][2]int{{-1, 0}, {1, -1}, {2, 2}, {3, 3}, {4, 5}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var lines []patch.Line
			for _, kind := range tc.kinds {
				lines = append(lines, patch.Line{Kind: kind})
			}
			var got [][2]int
			pairs := unifiedLinePairs(lines)
			if tc.split {
				pairs = splitLinePairs(lines)
			}
			for left, right := range pairs {
				got = append(got, [2]int{left, right})
			}
			testutil.RequireEqual(t, slices.Equal(got, tc.want), true,
				"line pairs=%v, want %v", got, tc.want)
			// Stopping at any prefix must prevent further yields.
			for limit := 1; limit <= len(tc.want); limit++ {
				count := 0
				for range pairs {
					count++
					if count == limit {
						break
					}
				}
				testutil.RequireEqual(t, count, limit, "early termination")
			}
		})
	}
}
