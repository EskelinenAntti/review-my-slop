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
	m := testModel(coveragePatch(), nil, nil)
	m.currentPatch.Branch = "main"
	m.kind = patch.Branch
	var requested []patch.Kind
	m.setRefresh(func(kind patch.Kind) (patch.Patch, error) {
		requested = append(requested, kind)
		p := coveragePatch()
		p.Kind, p.Branch = kind, "main"
		p.Files[0].Metadata = []string{
			fmt.Sprintf("refresh-%d", len(requested)),
		}
		return p, nil
	})
	next, cmd := m.Update(tea.FocusMsg{})
	m = requireModel(t, next)
	testutil.RequireEqual(t, cmd != nil, true, "focus did not refresh")

	m = updateModel(t, m, cmd())
	next, cmd = m.Update(textKey("R"))
	m = requireModel(t, next)
	testutil.RequireEqual(t, cmd != nil, true, "R did not refresh")

	m = updateModel(t, m, cmd())
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
	m := testModel(coveragePatch(), nil, nil)
	m.currentPatch.Branch = "main"
	m.setRefresh(func(kind patch.Kind) (patch.Patch, error) {
		p := coveragePatch()
		p.Kind, p.Branch = kind, "main"
		return p, nil
	})
	next, _ := m.Update(textKey("tab"))
	m = requireModel(t, next)
	testutil.RequireEqual(t, m.kind, patch.Branch,
		fmt.Sprintf("kind=%v", m.kind),
	)

	stale := coveragePatch()
	stale.Files[0].Metadata = []string{"stale"}
	m = updateModel(t, m, refreshDiffMsg{patch: stale})
	testutil.RequireEqual(t, !slices.Equal(
		m.currentPatch.Files[0].Metadata,
		[]string{"stale"},
	), true, "stale refresh applied",
	)

	m = updateModel(t, m, textKey("tab"))
	testutil.RequireEqual(t, m.kind, patch.Unstaged,
		fmt.Sprintf("kind=%v after toggling back to local", m.kind),
	)
}

func TestTabDoesNothingWithoutDefaultBranch(t *testing.T) {
	m := testModel(coveragePatch(), nil, nil)
	refreshed := false
	m.setRefresh(func(patch.Kind) (patch.Patch, error) {
		refreshed = true
		return coveragePatch(), nil
	})
	next, cmd := m.Update(textKey("tab"))
	m = requireModel(t, next)
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
	m := testModel(patch.Patch{}, nil, nil)
	m = updateModel(t, m, refreshDiffMsg{patch: coveragePatch()})
	testutil.RequireEqual(t, lineText(m), "package main",
		fmt.Sprintf("initial focus=%q", lineText(m)),
	)

	focusLine(t, &m, "new()")
	changed := coveragePatch()
	changed.Files[0].Hunks[0].Lines[2].Text = "different()"
	m = updateModel(t, m, refreshDiffMsg{patch: changed})
	if _, _, ok := m.diffView.Current(); !ok {
		t.Fatal("refresh lost focus")
	}
}

func TestCommentAfterRefreshUsesCurrentPatch(t *testing.T) {
	t.Setenv("EDITOR", "true")
	var saved patch.Patch
	m := testModel(coveragePatch(), nil, func(
		stored comments.Comment,
		p patch.Patch,
	) (comments.Comment, error) {
		saved = p
		return stored, nil
	})
	refreshed := coveragePatch()
	refreshed.Files[0].Metadata = []string{"refreshed"}
	m = updateModel(t, m, refreshDiffMsg{patch: refreshed})
	m = updateModel(t, m, textKey("c"))
	_ = updateModel(t, m, commentEditorFinishedMsg{body: "comment"})
	testutil.RequireEqual(t,
		slices.Equal(saved.Files[0].Metadata, []string{"refreshed"}), true,
		fmt.Sprintf("metadata=%q", saved.Files[0].Metadata),
	)
}
