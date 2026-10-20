package diffscreen

import (
	"math"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestScrollOffset(t *testing.T) {
	tests := []struct {
		name                    string
		offset, distance, limit int
		want                    int
	}{
		{name: "forward", offset: 5, distance: 3, limit: 10, want: 8},
		{name: "backward", offset: 5, distance: -3, limit: 10, want: 2},
		{name: "stationary", offset: 5, limit: 10, want: 5},
		{name: "past end", offset: 5, distance: 20, limit: 10, want: 10},
		{name: "past start", offset: 5, distance: -20, limit: 10},
		{
			name: "maximum distance", offset: 5,
			distance: math.MaxInt, limit: 10, want: 10,
		},
		{name: "minimum distance", offset: 5, distance: math.MinInt, limit: 10},
		{name: "empty range", distance: math.MaxInt},
		{
			name: "maximum limit", offset: math.MaxInt - 1,
			distance: math.MaxInt, limit: math.MaxInt, want: math.MaxInt,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := scrollOffset(tt.offset, tt.distance, tt.limit)
			testutil.RequireEqual(t, got, tt.want, "scroll offset = %d", got)
		})
	}
}
