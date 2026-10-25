package diffscreen

import "testing"

func TestVisibleRows(t *testing.T) {
	v := &diffView{rows: []entry{
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
