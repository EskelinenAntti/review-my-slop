package app

import (
	"errors"
	"fmt"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/eskelinenantti/review-my-slop/internal/testutil"
	"github.com/eskelinenantti/review-my-slop/internal/ui/diffscreen"
)

func TestRefreshTranslatesCursorAndSelection(t *testing.T) {
	m := newTestModel(singleHunkPatch(), nil, nil)
	m.patchView.Move(diffscreen.NextLine)
	m.patchView.ToggleSelection()
	m.patchView.Move(diffscreen.NextLine)
	_, want, _ := m.patchView.Current()
	refreshed := singleHunkPatch()
	refreshed.Files[0].Metadata = []string{"new metadata"}
	m.updatePatch(refreshed)
	_, got, ok := m.patchView.Current()
	testutil.RequireEqual(t, ok, true,
		fmt.Sprintf("focused line=%#v, want %#v", got, want),
	)
	testutil.RequireEqual(t, got, want,
		fmt.Sprintf("focused line=%#v, want %#v", got, want),
	)

	_, lines, ok := m.patchView.Selected()
	testutil.RequireEqual(t, ok, true, fmt.Sprintf("selection=%#v", lines))
	testutil.RequireEqual(t, len(lines), 2,
		fmt.Sprintf("selection=%#v", lines),
	)
}

func TestUnchangedRefreshPreservesStateAndClearsError(t *testing.T) {
	p := longContextPatch()
	p.Files[0].OldPath, p.Files[0].NewPath = "long.go", "long.go"
	m := newModel(p, nil, nil, initialLayout{size: size{Width: 40, Height: 8}})
	for range 8 {
		m.patchView.Move(diffscreen.NextLine)
	}
	m.patchView.ToggleSelection()
	m.patchView.Move(diffscreen.NextLine)
	m.patchView.ScrollHorizontal(12)
	before := m.patchView.Render(nil)
	m.err = errors.New("previous refresh failed")
	m = applyTestMessage(t, m, patchRefreshMsg{patch: p})
	if got := m.patchView.Render(nil); got != before {
		t.Fatal("refresh changed presentation state")
	}
	testutil.RequireEqual(t, m.err, nil,
		fmt.Sprintf("refresh did not clear error: %v", m.err),
	)
}

func TestRefreshFailureRetainsView(t *testing.T) {
	m := newTestModel(singleHunkPatch(), nil, nil)
	before := m.patchView.Render(nil)
	m = applyTestMessage(t, m, patchRefreshMsg{err: errors.New("git failed")})
	testutil.RequireEqual(t, m.err != nil, true,
		fmt.Sprintf("patch=%#v error=%v", m.currentPatch, m.err),
	)
	testutil.RequireEqual(t, m.patchView.Render(nil), before,
		fmt.Sprintf("patch=%#v error=%v", m.currentPatch, m.err),
	)
	testutil.RequireEqual(t, m.currentPatch.Root, "/repo",
		fmt.Sprintf("patch=%#v error=%v", m.currentPatch, m.err),
	)
}

func TestViewSwitchPreservesSemanticCursor(t *testing.T) {
	m := newTestModel(singleHunkPatch(), nil, nil)
	m = applyTestMessage(t, m, tea.WindowSizeMsg{Width: 120, Height: 30})
	m.patchView.Move(diffscreen.NextLine)
	m.patchView.Move(diffscreen.NextLine)
	_, want, _ := m.patchView.Current()
	m.setSideBySide(true)
	_, got, ok := m.patchView.Current()
	testutil.RequireEqual(t, ok, true,
		fmt.Sprintf("focused line=%#v, want %#v", got, want),
	)
	testutil.RequireEqual(t, got, want,
		fmt.Sprintf("focused line=%#v, want %#v", got, want),
	)
}
