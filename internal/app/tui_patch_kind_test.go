package app

import (
	"slices"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestTabTogglesDefaultBranchAndIgnoresStaleRefresh(t *testing.T) {
	m := testModel(coveragePatch(), nil, nil)
	m.currentPatch.Branch = "main"
	m.refreshPatch = func(kind patch.Kind) (patch.Patch, error) {
		p := coveragePatch()
		p.Kind, p.Branch = kind, "main"
		return p, nil
	}
	next, _ := m.Update(textKey("tab"))
	m = requireModel(t, next)
	testutil.RequireEqual(t, m.kind, patch.Branch, "kind=%v", m.kind)

	stale := coveragePatch()
	stale.Files[0].Metadata = []string{"stale"}
	m = updateModel(t, m, patchRefreshFinishedMsg{patch: stale})
	testutil.RequireEqual(t,
		!slices.Equal(
			m.currentPatch.Files[0].Metadata,
			[]string{"stale"},
		), true,
		"%s", "stale refresh applied",
	)

	m = updateModel(t, m, textKey("tab"))
	testutil.RequireEqual(t, m.kind, patch.Unstaged,
		"kind=%v after toggling back to local", m.kind,
	)
}

func TestTabDoesNothingWithoutDefaultBranch(t *testing.T) {
	m := testModel(coveragePatch(), nil, nil)
	refreshed := false
	m.refreshPatch = func(patch.Kind) (patch.Patch, error) {
		refreshed = true
		return coveragePatch(), nil
	}
	next, cmd := m.Update(textKey("tab"))
	m = requireModel(t, next)
	testutil.RequireEqual(t, cmd == nil, true,
		"tab changed model without default branch: cmd=%v refreshed=%v kind=%v",
		cmd != nil, refreshed, m.kind,
	)
	testutil.RequireEqual(t, !refreshed, true,
		"tab changed model without default branch: cmd=%v refreshed=%v kind=%v",
		cmd != nil, refreshed, m.kind,
	)
	testutil.RequireEqual(t, m.kind, patch.Unstaged,
		"tab changed model without default branch: cmd=%v refreshed=%v kind=%v",
		cmd != nil, refreshed, m.kind,
	)
}
