package diffscreen

import "testing"

func TestRenderKeepsPresentationStateLocal(t *testing.T) {
	for _, split := range []bool{false, true} {
		name := "unified"
		if split {
			name = "split"
		}
		t.Run(name, func(t *testing.T) {
			v := newDiffView(testPatch(), Options{
				SideBySide: split,
				Dark:       true,
			})
			first := mustFirst(t, v)
			last, _ := v.last()
			selection, ok := v.selectionBetween(first, first)
			if !ok {
				t.Fatal("could not select first line")
			}
			for _, state := range []struct {
				name      string
				viewport  diffViewport
				cursor    diffCursor
				selection *diffSelection
			}{
				{"full", v.newViewport(121, 20), first, nil},
				{"selected", v.newViewport(81, 20), last, &selection},
				{
					name:     "scrolled",
					viewport: v.keepVisible(v.newViewport(101, 3), last),
					cursor:   last,
				},
				{
					name: "clamped",
					viewport: diffViewport{
						width: 101, height: 4, top: -1, leftColumn: -1,
					},
					cursor: first,
				},
			} {
				want := v.render(state.viewport, state.cursor, state.selection)
				t.Run(state.name, func(t *testing.T) {
					t.Parallel()
					for range 3 {
						got := v.render(
							state.viewport, state.cursor, state.selection,
						)
						if got != want {
							t.Fatalf("render changed:\ngot  %q\nwant %q",
								got, want,
							)
						}
					}
				})
			}
		})
	}
}
