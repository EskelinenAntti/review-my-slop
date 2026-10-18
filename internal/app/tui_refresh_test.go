package app

import (
	"fmt"
	"slices"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestFocusAndManualRefreshLoadCurrentView(t *testing.T) {
	m := testModel(coveragePatch(), nil, nil)
	m.currentPatch.Branch = "main"
	m.kind = patch.Branch
	var requested []patch.Kind
	m.refreshPatch = func(kind patch.Kind) (patch.Patch, error) {
		requested = append(requested, kind)
		p := coveragePatch()
		p.Kind, p.Branch = kind, "main"
		p.Files[0].Metadata = []string{
			fmt.Sprintf("refresh-%d", len(requested)),
		}
		return p, nil
	}
	next, cmd := m.Update(tea.FocusMsg{})
	m = requireModel(t, next)
	testutil.RequireEqual(t, cmd != nil, true, "%s", "focus did not refresh")

	m = updateModel(t, m, cmd())
	next, cmd = m.Update(textKey("R"))
	m = requireModel(t, next)
	testutil.RequireEqual(t, cmd != nil, true, "%s", "R did not refresh")

	m = updateModel(t, m, cmd())
	testutil.RequireEqual(t,
		slices.Equal(requested, []patch.Kind{patch.Branch, patch.Branch}), true,
		"requested=%v metadata=%q", requested, m.currentPatch.Files[0].Metadata,
	)
	testutil.RequireEqual(t,
		slices.Equal(
			m.currentPatch.Files[0].Metadata,
			[]string{"refresh-2"},
		), true,
		"requested=%v metadata=%q", requested, m.currentPatch.Files[0].Metadata,
	)
}

func TestSourceEditorCompletionRefreshesPatch(t *testing.T) {
	m := testModel(coveragePatch(), nil, nil)
	refreshed := coveragePatch()
	refreshed.Files[0].Metadata = []string{"after-editor"}
	m.refreshPatch = func(patch.Kind) (patch.Patch, error) {
		return refreshed, nil
	}

	next, cmd := m.Update(sourceEditorFinishedMsg{})
	m = requireModel(t, next)
	testutil.RequireEqual(t, cmd != nil, true, "%s",
		"editor completion did not refresh",
	)

	m = updateModel(t, m, cmd())
	testutil.RequireEqual(t,
		slices.Equal(
			m.currentPatch.Files[0].Metadata,
			[]string{"after-editor"},
		),
		true, "metadata=%q", m.currentPatch.Files[0].Metadata,
	)
}

func TestPatchRefreshFallbackAndEmptyPatch(t *testing.T) {
	m := testModel(patch.Patch{}, nil, nil)
	m = updateModel(t, m, patchRefreshFinishedMsg{patch: coveragePatch()})
	testutil.RequireEqual(t, lineText(m), "package main", "initial focus=%q",
		lineText(m),
	)

	focusLine(t, &m, "new()")
	changed := coveragePatch()
	changed.Files[0].Hunks[0].Lines[2].Text = "different()"
	m = updateModel(t, m, patchRefreshFinishedMsg{patch: changed})
	if _, _, ok := m.diffView.Current(); !ok {
		t.Fatal("refresh lost focus")
	}
}
