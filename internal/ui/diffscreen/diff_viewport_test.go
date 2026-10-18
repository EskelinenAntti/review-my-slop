package diffscreen

import (
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestAlignmentOffset(t *testing.T) {
	for _, height := range []int{1, 2, 5, 6} {
		for _, tt := range []struct {
			alignment Alignment
			want      int
		}{
			{Top, 0},
			{Center, height / 2},
			{Bottom, height - 1},
			{Alignment(255), 0},
		} {
			got := alignmentOffset(height, tt.alignment)
			testutil.RequireEqual(t, got, tt.want,
				"alignmentOffset(%d, %d) = %d, want %d",
				height, tt.alignment, got, tt.want,
			)
		}
	}
}
