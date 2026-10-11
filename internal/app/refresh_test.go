package app

import (
	"errors"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/eskelinenantti/review-my-slop/internal/comments"
	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/ui/diffscreen"
)

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
	requireEqual(
		t,
		ok && got == want, true,
		"focused line=%#v, want %#v",
		got,
		want,
	)
	_, lines, ok := m.diffView.Selected()
	requireEqual(t, ok && len(lines) == 2, true, "selection=%#v", lines)
}

func TestUnchangedRefreshPreservesStateAndClearsError(t *testing.T) {
	p := longModelPatch()
	p.Files[0].OldPath, p.Files[0].NewPath = "long.go", "long.go"
	m := newModel(
		p,
		nil,
		nil,
		initialLayout{size: size{Width: 40, Height: 8}},
	)
	for range 8 {
		m.diffView.Move(diffscreen.NextLine)
	}
	m.diffView.ToggleSelection()
	m.diffView.Move(diffscreen.NextLine)
	m.diffView.ScrollHorizontal(horizontalScrollColumns)
	before := m.diffView.Render(nil)
	m.err = errors.New("previous refresh failed")
	m = updateModel(t, m, refreshDiffMsg{patch: p})
	{
		got := m.diffView.Render(nil)
		requireEqual(
			t,
			got, before,
			valueFormat,
			"refresh changed presentation state",
		)
	}
	requireEqual(
		t,
		m.err == nil, true,
		"refresh did not clear error: %v",
		m.err,
	)
}

func TestRefreshFailureRetainsView(t *testing.T) {
	m := testModel(modelPatch(), nil, nil)
	before := m.diffView.Render(nil)
	m = updateModel(t, m, refreshDiffMsg{err: errors.New("git failed")})
	requireEqual(
		t,
		m.currentPatch.Root == "/repo" &&
			(m.err != nil && m.diffView.Render(nil) == before),
		true,
		"patch=%#v error=%v",
		m.currentPatch,
		m.err,
	)
}

func TestViewSwitchPreservesSemanticCursor(t *testing.T) {
	m := testModel(modelPatch(), nil, nil)
	m = updateModel(t, m, tea.WindowSizeMsg{Width: 120, Height: 30})
	m.diffView.Move(diffscreen.NextLine)
	m.diffView.Move(diffscreen.NextLine)
	_, want, _ := m.diffView.Current()
	m.setSideBySide(true)
	_, got, ok := m.diffView.Current()
	requireEqual(
		t,
		ok && got == want, true,
		"focused line=%#v, want %#v",
		got,
		want,
	)
}

func TestCommentSaveUsesPatchAndPreservesAnchor(t *testing.T) {
	var savedPatch patch.Patch
	m := testModel(
		modelPatch(),
		nil,
		func(
			stored comments.Comment,
			p patch.Patch,
		) (
			comments.Comment,
			error,
		) {
			savedPatch = p
			stored.ID = "1"
			return stored, nil
		},
	)
	m.edit.body = "comment"
	m.edit.anchor = comments.Anchor{FilePath: fixtureMainGo}
	m.finishCommentEdit()
	requireEqual(
		t,
		savedPatch.Root == "/repo" && len(m.comments.items) == 1 &&
			m.comments.items[0].Anchor.FilePath == fixtureMainGo,
		true,
		"saved patch/comments = %#v %#v",
		savedPatch,
		m.comments.items,
	)
}
