package app

import (
	"errors"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestPatchRefreshRetainsQueuedKindAndIgnoresStaleResults(t *testing.T) {
	for _, result := range []string{"success", "failure"} {
		t.Run(result, func(t *testing.T) {
			m := testModel(modelPatch(), nil, nil)
			m.refresh = func(kind patch.Kind) (patch.Patch, error) {
				testutil.RequireEqual(t, kind, patch.Unstaged,
					"refresh did not retain queued patch kind")
				if result == "failure" {
					return patch.Patch{}, errors.New("git failed")
				}
				refreshed := modelPatch()
				refreshed.Root = "/stale"
				return refreshed, nil
			}
			cmd := m.loadRefresh()
			testutil.RequireEqual(t, cmd != nil, true,
				"missing refresh command")
			m.kind = patch.Branch
			err := errors.New("current error")
			m.err = err
			before := m.diffView.Render(nil)
			m = updateModel(t, m, cmd())

			testutil.RequireEqual(t, m.err, err, "stale refresh changed error")
			testutil.RequireEqual(t, m.currentPatch.Root, "/repo",
				"stale refresh replaced patch")
			testutil.RequireEqual(t, m.diffView.Render(nil), before,
				"stale refresh changed patch presentation")
		})
	}
}
