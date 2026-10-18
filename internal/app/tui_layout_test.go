package app

import (
	"context"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestNewUsesSavedSideBySideForWideInitialSize(t *testing.T) {
	m := newModel(context.Background(), modelPatch(), nil, nil, initialLayout{
		sideBySide: true,
		dimensions: terminalDimensions{width: 120, height: 30},
	})
	testutil.RequireEqual(t, m.diffOptions.SideBySide, true,
		"sideBySide=%v render=%q", m.diffOptions.SideBySide, m.render(),
	)
	testutil.RequireEqual(t, strings.Contains(m.render(), "│"), true,
		"sideBySide=%v render=%q", m.diffOptions.SideBySide, m.render(),
	)
}

func TestNewKeepsSavedSideBySideInactiveForNarrowInitialSize(t *testing.T) {
	m := newModel(context.Background(), modelPatch(), nil, nil, initialLayout{
		sideBySide: true,
		dimensions: terminalDimensions{width: 80, height: 30},
	})
	testutil.RequireEqual(t, m.diffOptions.SideBySide, true,
		"sideBySide=%v render=%q", m.diffOptions.SideBySide, m.render(),
	)
	testutil.RequireEqual(t, !strings.Contains(m.render(), "│"), true,
		"sideBySide=%v render=%q", m.diffOptions.SideBySide, m.render(),
	)

	m = updateModel(t, m, tea.WindowSizeMsg{Width: 120, Height: 30})
	testutil.RequireEqual(t, m.diffOptions.SideBySide, true,
		"sideBySide=%v render=%q", m.diffOptions.SideBySide, m.render(),
	)
	testutil.RequireEqual(t, strings.Contains(m.render(), "│"), true,
		"sideBySide=%v render=%q", m.diffOptions.SideBySide, m.render(),
	)
}

func TestSideBySideToggleStillSavesPreference(t *testing.T) {
	var saved []bool
	m := newModel(context.Background(), modelPatch(), nil, nil, initialLayout{
		saveSideBySide: func(enabled bool) error {
			saved = append(saved, enabled)
			return nil
		},
		dimensions: terminalDimensions{width: 120, height: 30},
	})

	m = updateModel(t, m, textKey("t"))
	updateModel(t, m, textKey("t"))
	testutil.RequireEqual(t, slices.Equal(saved, []bool{true, false}), true,
		"saved=%v", saved,
	)
}

func TestNarrowLayoutToggleSavesPreferenceWithoutApplicationGeometryRules(
	t *testing.T,
) {
	var saved []bool
	m := newModel(context.Background(), modelPatch(), nil, nil, initialLayout{
		dimensions: terminalDimensions{width: 80, height: 30},
		saveSideBySide: func(enabled bool) error {
			saved = append(saved, enabled)
			return nil
		},
	})
	m = updateModel(t, m, textKey("t"))
	testutil.RequireEqual(t, m.err, nil,
		"narrow toggle: saved=%v err=%v render=%q", saved, m.err, m.render(),
	)
	testutil.RequireEqual(t, slices.Equal(saved, []bool{true}), true,
		"narrow toggle: saved=%v err=%v render=%q", saved, m.err, m.render(),
	)
	testutil.RequireEqual(t, !strings.Contains(m.render(), " │ "), true,
		"narrow toggle: saved=%v err=%v render=%q", saved, m.err, m.render(),
	)

	m = updateModel(t, m, tea.WindowSizeMsg{Width: 120, Height: 30})
	testutil.RequireEqual(t, strings.Contains(m.render(), " │ "), true, "%s",
		"widening did not activate preferred layout",
	)
}

func TestVimSequencesAndLayoutToggle(t *testing.T) {
	m := testModel(coveragePatch(), nil, nil)
	var saved []bool
	m.saveLayout = func(enabled bool) error {
		saved = append(saved, enabled)
		return nil
	}
	m.setSideBySide(false)
	m = updateModel(t, m, tea.WindowSizeMsg{Width: 120, Height: 20})
	m = updateModel(t, m, textKey("G"))
	testutil.RequireEqual(t, lineText(m), "more()", "G line=%q", lineText(m))

	m = updateModel(t, m, textKey("g"))
	m = updateModel(t, m, textKey("g"))
	testutil.RequireEqual(t, lineText(m), "package main", "gg line=%q",
		lineText(m),
	)

	m = updateModel(t, m, textKey("t"))
	testutil.RequireEqual(t, m.diffOptions.SideBySide, true, "%s",
		"split view not enabled",
	)
	testutil.RequireEqual(t, strings.Contains(m.render(), "│"), true, "%s",
		"split view not enabled",
	)

	m = updateModel(t, m, textKey("t"))
	testutil.RequireEqual(t, !m.diffOptions.SideBySide, true, "saved=%v", saved)
	testutil.RequireEqual(t, slices.Equal(saved, []bool{true, false}), true,
		"saved=%v", saved,
	)
}

func TestSavedSideBySideCanBeDisabledInNarrowTerminal(t *testing.T) {
	m := testModel(coveragePatch(), nil, nil)
	var saved []bool
	m.saveLayout = func(enabled bool) error {
		saved = append(saved, enabled)
		return nil
	}
	m.setSideBySide(true)
	m = updateModel(t, m, tea.WindowSizeMsg{Width: 80, Height: 20})
	m = updateModel(t, m, textKey("t"))
	testutil.RequireEqual(t, !m.diffOptions.SideBySide, true,
		"sideBySide=%v saved=%v", m.diffOptions.SideBySide, saved,
	)
	testutil.RequireEqual(t, slices.Equal(saved, []bool{false}), true,
		"sideBySide=%v saved=%v", m.diffOptions.SideBySide, saved,
	)
}

func TestResizeAcrossSideBySideThresholdPreservesFocusedLine(t *testing.T) {
	m := testModel(coveragePatch(), nil, nil)
	m.setSideBySide(true)
	focusLine(t, &m, "keep()")
	_, before, _ := m.diffView.Current()
	m = updateModel(t, m, tea.WindowSizeMsg{Width: 80, Height: 20})
	m = updateModel(t, m, tea.WindowSizeMsg{Width: 120, Height: 20})
	_, got, ok := m.diffView.Current()
	testutil.RequireEqual(t, ok, true, "focused line=%#v, want %#v", got,
		before,
	)
	testutil.RequireEqual(t, got, before, "focused line=%#v, want %#v", got,
		before,
	)
}

func TestSideBySidePaneSwitchingUsesCtrlWSequences(t *testing.T) {
	m := testModel(coveragePatch(), nil, nil)
	m = updateModel(t, m, tea.WindowSizeMsg{Width: 120, Height: 30})
	m.setSideBySide(true)
	focusLine(t, &m, "new()")
	m = updateModel(t, m, controlKey('w'))
	m = updateModel(t, m, textKey("h"))
	testutil.RequireEqual(t, lineText(m), "old()", "old pane line=%q",
		lineText(m),
	)

	m = updateModel(t, m, controlKey('w'))
	m = updateModel(t, m, controlKey('w'))
	testutil.RequireEqual(t, lineText(m), "new()", "new pane line=%q",
		lineText(m),
	)
}
