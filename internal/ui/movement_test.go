package ui

import (
	"math"
	"testing"
)

func TestMoveWithinBounds(t *testing.T) {
	for _, tt := range []struct {
		name                    string
		current, delta, maximum int
		want                    int
	}{
		{name: "forward", current: 4, delta: 3, maximum: 10, want: 7},
		{name: "backward", current: 4, delta: -3, maximum: 10, want: 1},
		{name: "stationary", current: 4, maximum: 10, want: 4},
		{name: "past start", current: 4, delta: -5, maximum: 10},
		{name: "past end", current: 4, delta: 7, maximum: 10, want: 10},
		{name: "minimum delta", current: 4, delta: math.MinInt, maximum: 10},
		{
			name:    "maximum delta",
			current: 4, delta: math.MaxInt, maximum: 10, want: 10,
		},
		{name: "empty range", delta: math.MaxInt},
		{name: "shrunken range", current: 12, maximum: 10, want: 10},
		{
			name:    "backward after shrink",
			current: 12, delta: -5, maximum: 10, want: 7,
		},
		{
			name:    "maximum offset",
			current: math.MaxInt, delta: math.MaxInt,
			maximum: math.MaxInt, want: math.MaxInt,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := MoveWithinBounds(tt.current, tt.delta, tt.maximum)
			if got != tt.want {
				t.Errorf("MoveWithinBounds(%d, %d, %d) = %d, want %d",
					tt.current, tt.delta, tt.maximum, got, tt.want,
				)
			}
		})
	}
}
