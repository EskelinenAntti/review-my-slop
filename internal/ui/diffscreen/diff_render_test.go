package diffscreen

import (
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestRenderKeepsPresentationStateLocal(t *testing.T) {
	for _, tc := range []struct {
		name  string
		split bool
	}{
		{name: "unified"},
		{name: "split", split: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := newDiffView(testPatch(), Options{
				SideBySide: tc.split,
				Dark:       true,
			})
			viewport := v.newViewport(100, 20)
			first, _ := v.first()
			last, _ := v.last()
			selection := beginSelection(first)
			originalSelection := selection
			initial := v.render(viewport, first, &selection)

			other := v.render(v.newViewport(120, 5), last, nil)
			if other == initial {
				t.Fatal("different render state produced identical output")
			}
			testutil.RequireEqual(t,
				v.render(viewport, first, &selection), initial,
				"render retained state from another call")
			testutil.RequireEqual(t, selection, originalSelection,
				"render changed the selection")

			unclamped := viewport
			unclamped.top, unclamped.leftColumn = -10, -10
			testutil.RequireEqual(t,
				v.render(unclamped, first, &selection), initial,
				"render did not clamp the viewport before rendering")
		})
	}
}
