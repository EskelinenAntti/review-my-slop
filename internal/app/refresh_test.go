package app

import (
	"fmt"
	"slices"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/eskelinenantti/review-my-slop/internal/comments"
	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestFocusAndManualRefreshLoadCurrentView(t *testing.T) {
	m := newTestModel(multiHunkPatch(), nil, nil)
	m.currentPatch.Branch = "main"
	m.kind = patch.Branch
	var requested []patch.Kind
	m.setPatchRefresh(func(kind patch.Kind) (patch.Patch, error) {
		requested = append(requested, kind)
		p := multiHunkPatch()
		p.Kind, p.Branch = kind, "main"
		p.Files[0].Metadata = []string{
			fmt.Sprintf("refresh-%d", len(requested)),
		}
		return p, nil
	})
	next, cmd := m.Update(tea.FocusMsg{})
	m = requireAppModel(t, next)
	testutil.RequireEqual(t, cmd != nil, true, "focus did not refresh")

	m = applyTestMessage(t, m, cmd())
	next, cmd = m.Update(textKeyMessage("R"))
	m = requireAppModel(t, next)
	testutil.RequireEqual(t, cmd != nil, true, "R did not refresh")

	m = applyTestMessage(t, m, cmd())
	testutil.RequireEqual(t,
		slices.Equal(requested, []patch.Kind{patch.Branch, patch.Branch}),
		true, fmt.Sprintf("requested=%v metadata=%q", requested,
			m.currentPatch.Files[0].Metadata,
		),
	)
	testutil.RequireEqual(t, slices.Equal(
		m.currentPatch.Files[0].Metadata,
		[]string{"refresh-2"},
	), true, fmt.Sprintf("requested=%v metadata=%q", requested,
		m.currentPatch.Files[0].Metadata,
	),
	)
}

func TestTabTogglesDefaultBranchAndIgnoresStaleRefresh(t *testing.T) {
	m := newTestModel(multiHunkPatch(), nil, nil)
	m.currentPatch.Branch = "main"
	m.setPatchRefresh(func(kind patch.Kind) (patch.Patch, error) {
		p := multiHunkPatch()
		p.Kind, p.Branch = kind, "main"
		return p, nil
	})
	next, _ := m.Update(textKeyMessage("tab"))
	m = requireAppModel(t, next)
	testutil.RequireEqual(t, m.kind, patch.Branch,
		fmt.Sprintf("kind=%v", m.kind),
	)

	stale := multiHunkPatch()
	stale.Files[0].Metadata = []string{"stale"}
	m = applyTestMessage(t, m, patchRefreshMsg{patch: stale})
	testutil.RequireEqual(t, !slices.Equal(
		m.currentPatch.Files[0].Metadata,
		[]string{"stale"},
	), true, "stale refresh applied",
	)

	m = applyTestMessage(t, m, textKeyMessage("tab"))
	testutil.RequireEqual(t, m.kind, patch.Unstaged,
		fmt.Sprintf("kind=%v after toggling back to local", m.kind),
	)
}

func TestTabDoesNothingWithoutDefaultBranch(t *testing.T) {
	m := newTestModel(multiHunkPatch(), nil, nil)
	refreshed := false
	m.setPatchRefresh(func(patch.Kind) (patch.Patch, error) {
		refreshed = true
		return multiHunkPatch(), nil
	})
	next, cmd := m.Update(textKeyMessage("tab"))
	m = requireAppModel(t, next)
	testutil.RequireEqual(t, cmd == nil, true,
		fmt.Sprintf("tab without default branch: cmd=%v refreshed=%v kind=%v",
			cmd != nil, refreshed, m.kind,
		),
	)
	testutil.RequireEqual(t, !refreshed, true,
		fmt.Sprintf("tab without default branch: cmd=%v refreshed=%v kind=%v",
			cmd != nil, refreshed, m.kind,
		),
	)
	testutil.RequireEqual(t, m.kind, patch.Unstaged,
		fmt.Sprintf("tab without default branch: cmd=%v refreshed=%v kind=%v",
			cmd != nil, refreshed, m.kind,
		),
	)
}

func TestDiffRefreshFallbackAndEmptyDiff(t *testing.T) {
	m := newTestModel(patch.Patch{}, nil, nil)
	m = applyTestMessage(t, m, patchRefreshMsg{patch: multiHunkPatch()})
	testutil.RequireEqual(t, focusedLineText(m), "package main",
		fmt.Sprintf("initial focus=%q", focusedLineText(m)),
	)

	focusPatchLine(t, &m, "new()")
	changed := multiHunkPatch()
	changed.Files[0].Hunks[0].Lines[2].Text = "different()"
	m = applyTestMessage(t, m, patchRefreshMsg{patch: changed})
	if _, _, ok := m.patchView.Current(); !ok {
		t.Fatal("refresh lost focus")
	}
}

func TestCommentAfterRefreshUsesCurrentPatch(t *testing.T) {
	t.Setenv("EDITOR", "true")
	var saved patch.Patch
	m := newTestModel(multiHunkPatch(), nil, func(
		stored comments.Comment,
		p patch.Patch,
	) (comments.Comment, error) {
		saved = p
		return stored, nil
	})
	refreshed := multiHunkPatch()
	refreshed.Files[0].Metadata = []string{"refreshed"}
	m = applyTestMessage(t, m, patchRefreshMsg{patch: refreshed})
	m = applyTestMessage(t, m, textKeyMessage("c"))
	_ = applyTestMessage(t, m, commentEditorFinishedMsg{body: "comment"})
	testutil.RequireEqual(t,
		slices.Equal(saved.Files[0].Metadata, []string{"refreshed"}), true,
		fmt.Sprintf("metadata=%q", saved.Files[0].Metadata),
	)
}
