package diffscreen

import (
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

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
