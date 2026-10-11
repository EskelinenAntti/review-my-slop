package app

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/eskelinenantti/review-my-slop/internal/comments"
	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

func TestFocusAndManualRefreshLoadCurrentView(t *testing.T) {
	m := testModel(coveragePatch(), nil, nil)
	m.currentPatch.Branch = "main"
	m.kind = patch.Branch
	var requested []patch.Kind
	m.setRefresh(
		func(kind patch.Kind) (patch.Patch, error) {
			requested = append(requested, kind)
			p := coveragePatch()
			p.Kind, p.Branch = kind, "main"
			p.Files[0].Metadata = []string{
				fmt.Sprintf("refresh-%d", len(requested)),
			}
			return p, nil
		},
	)
	next, cmd := m.Update(tea.FocusMsg{})
	m = requireType[model](t, next)
	requireEqual(t, cmd != nil, true, valueFormat, "focus did not refresh")
	m = updateModel(t, m, cmd())
	next, cmd = m.Update(textKey("R"))
	m = requireType[model](t, next)
	requireEqual(t, cmd != nil, true, valueFormat, "R did not refresh")
	m = updateModel(t, m, cmd())
	requireEqual(
		t,
		slices.Equal(requested, []patch.Kind{patch.Branch, patch.Branch}) &&
			slices.Equal(
				m.currentPatch.Files[0].Metadata,
				[]string{"refresh-2"},
			),
		true,
		"requested=%v metadata=%q",
		requested,
		m.currentPatch.Files[0].Metadata,
	)
}

func TestSourceEditorCompletionRefreshesDiff(t *testing.T) {
	m := testModel(coveragePatch(), nil, nil)
	refreshed := coveragePatch()
	refreshed.Files[0].Metadata = []string{"after-editor"}
	m.setRefresh(
		func(patch.Kind) (patch.Patch, error) {
			return refreshed, nil
		},
	)

	next, cmd := m.Update(sourceEditorFinishedMsg{})
	m = requireType[model](t, next)
	requireEqual(
		t,
		cmd != nil,
		true,
		valueFormat,
		"editor completion did not refresh",
	)
	m = updateModel(t, m, cmd())
	requireEqual(
		t,
		slices.Equal(
			m.currentPatch.Files[0].Metadata,
			[]string{"after-editor"},
		),
		true,
		"metadata=%q",
		m.currentPatch.Files[0].Metadata,
	)
}

func TestHeaderShowsAddedAndRemovedLineCounts(t *testing.T) {
	header := strings.SplitN(
		ansi.Strip(testModel(coveragePatch(), nil, nil).render()),
		newline,
		2,
	)[0]
	requireEqual(t, header, "review-my-slop  +2-1", "header = %q", header)
}

func TestSearchMovesIncrementallyRepeatsAndRestoresOrigin(t *testing.T) {
	m := testModel(coveragePatch(), nil, nil)
	origin := screenBody(m.diffView.Render(nil))
	m = updateModel(t, m, textKey(searchKey))
	m = updateModel(t, m, textKey("keep"))
	first := screenBody(m.diffView.Render(nil))
	requireEqual(
		t,
		m.mode == modeSearch && lineText(m) == fixtureKeep, true,
		"mode=%v line=%q",
		m.mode,
		lineText(m),
	)
	m = updateModel(t, m, specialKey(tea.KeyEnter))
	m = updateModel(t, m, textKey("n"))
	requireEqual(
		t,
		screenBody(m.diffView.Render(nil)) != first &&
			lineText(m) == fixtureKeep,
		true,
		"next=%#v",
		screenBody(m.diffView.Render(nil)),
	)
	m = updateModel(t, m, textKey("N"))
	requireEqual(
		t,
		screenBody(m.diffView.Render(nil)), first,
		"previous=%#v",
		screenBody(m.diffView.Render(nil)),
	)
	m = updateModel(t, m, textKey(searchKey))
	m = updateModel(t, m, textKey("missing"))
	requireEqual(
		t,
		strings.Contains(m.render(), "no matches"),
		true,
		valueFormat,
		"missing search did not miss",
	)
	m = updateModel(t, m, specialKey(tea.KeyEsc))
	requireEqual(
		t,
		screenBody(m.diffView.Render(nil)), first,
		"cancel=%#v origin=%#v",
		screenBody(m.diffView.Render(nil)),
		origin,
	)
}

func TestSearchMatchesFileNamesAndBackspaceRestoresOrigin(t *testing.T) {
	m := testModel(coveragePatch(), nil, nil)
	origin := screenBody(m.diffView.Render(nil))
	m = updateModel(t, m, textKey(searchKey))
	m = updateModel(t, m, textKey(fixtureMainGo))
	file, _, _ := m.diffView.Current()
	requireEqual(
		t,
		file.DisplayPath, fixtureMainGo,
		"file=%q",
		file.DisplayPath,
	)
	for range len(fixtureMainGo) {
		m = updateModel(t, m, specialKey(tea.KeyBackspace))
	}
	requireEqual(
		t,
		screenBody(m.diffView.Render(nil)), origin,
		"diffCursor=%#v",
		screenBody(m.diffView.Render(nil)),
	)
}

func TestSideBySideSearchActivatesPaneAndCancelRestoresIt(t *testing.T) {
	m := testModel(coveragePatch(), nil, nil)
	m = updateModel(
		t,
		m,
		tea.WindowSizeMsg{Width: 120, Height: defaultHeight},
	)
	m.setSideBySide(true)
	origin := screenBody(m.diffView.Render(nil))
	m = updateModel(t, m, textKey(searchKey))
	m = updateModel(t, m, textKey(fixtureOld))
	requireEqual(
		t,
		lineText(m), fixtureOld,
		"diffCursor=%#v line=%q",
		screenBody(m.diffView.Render(nil)),
		lineText(m),
	)
	m = updateModel(t, m, specialKey(tea.KeyEsc))
	requireEqual(
		t,
		screenBody(m.diffView.Render(nil)), origin,
		"cancel=%#v want=%#v",
		screenBody(m.diffView.Render(nil)),
		origin,
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
	next, _ := m.Update(textKey(fixtureTab))
	m = requireType[model](t, next)
	requireEqual(t, m.kind, patch.Branch, "kind=%v", m.kind)
	stale := coveragePatch()
	stale.Files[0].Metadata = []string{"stale"}
	m = updateModel(t, m, refreshDiffMsg{patch: stale})
	requireEqual(
		t,
		!slices.Equal(m.currentPatch.Files[0].Metadata, []string{"stale"}),
		true,
		valueFormat,
		"stale refresh applied",
	)
	m = updateModel(t, m, textKey(fixtureTab))
	requireEqual(
		t,
		m.kind, patch.Unstaged,
		"kind=%v after toggling back to local",
		m.kind,
	)
}

func TestTabDoesNothingWithoutDefaultBranch(t *testing.T) {
	m := testModel(coveragePatch(), nil, nil)
	refreshed := false
	m.setRefresh(func(patch.Kind) (patch.Patch, error) {
		refreshed = true
		return coveragePatch(), nil
	})
	next, cmd := m.Update(textKey(fixtureTab))
	m = requireType[model](t, next)
	requireEqual(
		t,
		cmd == nil && !refreshed && m.kind == patch.Unstaged, true,
		"tab changed model without default branch"+
			": cmd=%v refreshed=%v kind=%v",
		cmd != nil,
		refreshed,
		m.kind,
	)
}

func TestDiffRefreshFallbackAndEmptyDiff(t *testing.T) {
	m := testModel(patch.Patch{}, nil, nil)
	m = updateModel(t, m, refreshDiffMsg{patch: coveragePatch()})
	requireEqual(
		t,
		lineText(m), fixturePackageMain,
		"initial focus=%q",
		lineText(m),
	)
	focusLine(t, &m, fixtureNew)
	changed := coveragePatch()
	changed.Files[0].Hunks[0].Lines[2].Text = "different()"
	m = updateModel(t, m, refreshDiffMsg{patch: changed})
	{
		_, _, ok := m.diffView.Current()
		requireEqual(t, ok, true, valueFormat, "refresh lost focus")
	}
}

func TestCommentAfterRefreshUsesCurrentPatch(t *testing.T) {
	t.Setenv(editorVariable, fixtureTrue)
	var saved patch.Patch
	m := testModel(
		coveragePatch(),
		nil,
		func(
			stored comments.Comment,
			p patch.Patch,
		) (
			comments.Comment,
			error,
		) {
			saved = p
			return stored, nil
		},
	)
	refreshed := coveragePatch()
	refreshed.Files[0].Metadata = []string{"refreshed"}
	m = updateModel(t, m, refreshDiffMsg{patch: refreshed})
	m = updateModel(t, m, textKey(commentKey))
	_ = updateModel(t, m, commentEditorFinishedMsg{body: "comment"})
	requireEqual(
		t,
		slices.Equal(saved.Files[0].Metadata, []string{"refreshed"}),
		true,
		"metadata=%q",
		saved.Files[0].Metadata,
	)
}

func TestViewPreservesTerminalColors(t *testing.T) {
	result := testModel(coveragePatch(), nil, nil).View()
	requireEqual(
		t,
		result.BackgroundColor == nil && result.ForegroundColor == nil &&
			result.AltScreen &&
			result.ReportFocus,
		true,
		"view=%#v",
		result,
	)
	requireEqual(
		t,
		result.MouseMode, tea.MouseModeCellMotion,
		valueFormat,
		"view does not capture terminal wheel events",
	)
}
