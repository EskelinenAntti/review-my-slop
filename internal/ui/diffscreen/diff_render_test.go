package diffscreen

import (
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestRenderStateIsLocalToEachCall(t *testing.T) {
	t.Run("unified", func(t *testing.T) {
		assertRenderStateIsLocal(t, newUnifiedView(longPatch(), true))
	})
	t.Run("split", func(t *testing.T) {
		assertRenderStateIsLocal(t, newSideBySideView(longPatch(), true))
	})
}

func assertRenderStateIsLocal(t *testing.T, v *diffView) {
	t.Helper()
	first := mustFirst(t, v)
	next, ok := v.move(first, forward)
	testutil.RequireEqual(t, ok, true, "second line missing")
	viewport := v.newViewport(80, 5)
	baseline := v.render(viewport, first, nil)
	selection := beginSelection(next)
	scrolled := v.scrollHorizontal(viewport, 3)
	for _, state := range []struct {
		name      string
		viewport  diffViewport
		cursor    diffCursor
		selection *diffSelection
	}{
		{"cursor", viewport, next, nil},
		{"selection", viewport, first, &selection},
		{"scroll", scrolled, first, nil},
		{"resize", v.resize(viewport, 70, 4), first, nil},
	} {
		t.Run(state.name, func(t *testing.T) {
			rendered := v.render(
				state.viewport, state.cursor, state.selection,
			)
			testutil.RequireEqual(t, rendered != baseline, true,
				"render ignored changed %s", state.name)
			testutil.RequireEqual(t,
				v.render(viewport, first, nil), baseline,
				"render retained changed %s", state.name)
		})
	}
}
