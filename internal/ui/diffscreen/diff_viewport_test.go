package diffscreen

import (
	"math"
	"testing"
)

func TestClampedScrollOffset(t *testing.T) {
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
			got := clampedScrollOffset(tt.current, tt.delta, tt.maximum)
			if got != tt.want {
				t.Errorf("clampedScrollOffset(%d, %d, %d) = %d, want %d",
					tt.current, tt.delta, tt.maximum, got, tt.want,
				)
			}
		})
	}
}

func TestStickyHeaderHeight(t *testing.T) {
	v := &diffView{rows: []projectedRow{
		{kind: fileRow},
		{kind: hunkRow},
		{kind: metadataRow},
		{kind: lineRow},
		{kind: fileRow},
	}}
	for _, tt := range []struct {
		name        string
		top, height int
		want        int
	}{
		{name: "visible file header", top: 0, height: 3},
		{name: "hunk", top: 1, height: 3, want: 1},
		{name: "metadata", top: 2, height: 3, want: 1},
		{name: "code", top: 3, height: 3, want: 1},
		{name: "next file header", top: 4, height: 3},
		{name: "single row", top: 3, height: 1},
		{name: "no height", top: 3, height: 0},
		{name: "before rows", top: -1, height: 3},
		{name: "after rows", top: 5, height: 3},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := v.stickyHeaderHeight(tt.top, tt.height); got != tt.want {
				t.Errorf("stickyHeaderHeight = %d, want %d", got, tt.want)
			}
		})
	}
	if got := (&diffView{}).stickyHeaderHeight(0, 3); got != 0 {
		t.Errorf("empty view stickyHeaderHeight = %d, want 0", got)
	}
}

func TestVisibleRows(t *testing.T) {
	v := &diffView{rows: []projectedRow{
		{kind: fileRow},
		{kind: hunkRow},
		{kind: lineRow},
		{kind: lineRow},
		{kind: lineRow},
	}}
	for _, tt := range []struct {
		name        string
		top, height int
		start, end  int
	}{
		{name: "file header in content", top: 0, height: 3, start: 0, end: 3},
		{
			name: "sticky header reserves a row",
			top:  1, height: 3, start: 1, end: 3,
		},
		{
			name: "single row has no sticky header",
			top:  2, height: 1, start: 2, end: 3,
		},
		{
			name: "content ends before viewport",
			top:  3, height: 5, start: 3, end: 5,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			viewport := diffViewport{top: tt.top, height: tt.height}
			start, end := v.visibleRows(viewport)
			if start != tt.start || end != tt.end {
				t.Fatalf("visibleRows = [%d, %d), want [%d, %d)",
					start, end, tt.start, tt.end,
				)
			}
			for row := -1; row <= len(v.rows); row++ {
				want := row >= tt.start && row < tt.end
				if got := v.rowVisible(viewport, row); got != want {
					t.Errorf("rowVisible(%d) = %v, want %v", row, got, want)
				}
			}
		})
	}
}

func TestVisibleRowsEmptyView(t *testing.T) {
	v := &diffView{}
	viewport := diffViewport{height: 5}
	start, end := v.visibleRows(viewport)
	if start != 0 || end != 0 {
		t.Fatalf("visibleRows = [%d, %d), want [0, 0)", start, end)
	}
	if v.rowVisible(viewport, 0) {
		t.Fatal("empty view has a visible row")
	}
	if progress := v.viewportProgress(viewport); progress != 0 {
		t.Fatalf("empty viewport progress = %d, want 0", progress)
	}
}
