package app

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
	"github.com/eskelinenantti/review-my-slop/internal/ui/diffscreen"
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

func TestRefreshTranslatesCursorAndSelection(t *testing.T) {
	m := testModel(modelPatch(), nil, nil)
	m.diffView.Move(diffscreen.NextLine)
	m.diffView.ToggleSelection()
	m.diffView.Move(diffscreen.NextLine)
	_, want, _ := m.diffView.Current()
	refreshed := modelPatch()
	refreshed.Files[0].Metadata = []string{"new metadata"}
	m.updatePatch(refreshed)
	_, got, ok := m.diffView.Current()
	testutil.RequireEqual(t, ok, true, "focused line=%#v, want %#v", got, want)
	testutil.RequireEqual(t, got, want, "focused line=%#v, want %#v", got, want)

	_, lines, ok := m.diffView.Selected()
	testutil.RequireEqual(t, ok, true, "selection=%#v", lines)
	testutil.RequireEqual(t, len(lines), 2, "selection=%#v", lines)
}

func TestUnchangedRefreshPreservesStateAndClearsError(t *testing.T) {
	p := longModelPatch()
	p.Files[0].OldPath, p.Files[0].NewPath = "long.go", "long.go"
	m := newModel(
		context.Background(),
		p,
		nil,
		nil,
		initialLayout{dimensions: terminalDimensions{width: 40, height: 8}},
	)
	for range 8 {
		m.diffView.Move(diffscreen.NextLine)
	}
	m.diffView.ToggleSelection()
	m.diffView.Move(diffscreen.NextLine)
	m.diffView.ScrollHorizontal(12)
	before := m.diffView.Render(nil)
	m.err = errors.New("previous refresh failed")
	m = updateModel(t, m, patchRefreshFinishedMsg{patch: p})
	if got := m.diffView.Render(nil); got != before {
		t.Fatal("refresh changed presentation state")
	}
	testutil.RequireEqual(t, m.err, nil, "refresh did not clear error: %v",
		m.err,
	)
}

func TestRefreshFailureRetainsView(t *testing.T) {
	m := testModel(modelPatch(), nil, nil)
	before := m.diffView.Render(nil)
	m = updateModel(
		t,
		m,
		patchRefreshFinishedMsg{err: errors.New("git failed")},
	)
	testutil.RequireEqual(t, m.err != nil, true, "patch=%#v error=%v",
		m.currentPatch, m.err,
	)
	testutil.RequireEqual(t, m.diffView.Render(nil), before,
		"patch=%#v error=%v", m.currentPatch, m.err,
	)
	testutil.RequireEqual(t, m.currentPatch.Root, "/repo", "patch=%#v error=%v",
		m.currentPatch, m.err,
	)
}
