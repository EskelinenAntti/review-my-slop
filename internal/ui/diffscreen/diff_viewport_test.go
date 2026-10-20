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

func TestViewportRowAt(t *testing.T) {
	v := &diffView{rows: []entry{
		{kind: fileRow},
		{kind: hunkRow},
		{kind: lineRow},
		{kind: lineRow},
		{kind: fileRow},
		{kind: lineRow},
	}}
	for _, tt := range []struct {
		name                 string
		top, height, bodyRow int
		wantRow              int
		wantOK               bool
	}{
		{name: "file header", height: 3, wantOK: true},
		{
			name: "unscrolled code", height: 3, bodyRow: 2,
			wantRow: 2, wantOK: true,
		},
		{name: "above body", height: 3, bodyRow: -1},
		{name: "below body", height: 3, bodyRow: 3},
		{name: "sticky header", top: 1, height: 3},
		{
			name: "below sticky header", top: 1, height: 3, bodyRow: 1,
			wantRow: 1, wantOK: true,
		},
		{
			name: "last sticky content row", top: 1, height: 3, bodyRow: 2,
			wantRow: 2, wantOK: true,
		},
		{name: "below sticky body", top: 1, height: 3, bodyRow: 3},
		{
			name: "single row has no sticky header", top: 2, height: 1,
			wantRow: 2, wantOK: true,
		},
		{
			name: "scrolled file header", top: 4, height: 3,
			wantRow: 4, wantOK: true,
		},
		{name: "padding", top: 4, height: 3, bodyRow: 2},
		{name: "empty height", height: 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			viewport := diffViewport{top: tt.top, height: tt.height}
			row, ok := v.rowAt(viewport, tt.bodyRow)
			testutil.RequireEqual(t, ok, tt.wantOK, "body row validity")
			testutil.RequireEqual(t, row, tt.wantRow, "projected row")
		})
	}
}

func TestViewportRowAtEmptyPatch(t *testing.T) {
	v := &diffView{}
	_, ok := v.rowAt(diffViewport{height: 3}, 0)
	testutil.RequireEqual(t, ok, false, "empty patch has no projected rows")
}

func TestViewportContentEnd(t *testing.T) {
	v := &diffView{rows: []entry{
		{kind: fileRow},
		{kind: hunkRow},
		{kind: lineRow},
		{kind: lineRow},
		{kind: fileRow},
		{kind: lineRow},
	}}
	for _, tt := range []struct {
		name             string
		top, height, end int
	}{
		{name: "file header", height: 3, end: 3},
		{name: "sticky header", top: 1, height: 3, end: 3},
		{name: "single row", top: 2, height: 1, end: 3},
		{name: "padding", top: 4, height: 5, end: 6},
		{name: "sticky padding", top: 5, height: 3, end: 6},
	} {
		t.Run(tt.name, func(t *testing.T) {
			viewport := diffViewport{top: tt.top, height: tt.height}
			testutil.RequireEqual(t, v.contentEnd(viewport), tt.end,
				"content end")
		})
	}
	testutil.RequireEqual(t, (&diffView{}).contentEnd(diffViewport{height: 3}),
		0, "empty patch content end")
}
