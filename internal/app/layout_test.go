package app

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestNewUsesSavedSideBySideForWideInitialSize(t *testing.T) {
	m := newModel(singleHunkPatch(), nil, nil, initialLayout{
		SideBySide: true,
		size:       size{Width: 120, Height: 30},
	})
	testutil.RequireEqual(t, m.patchViewOptions.SideBySide, true,
		fmt.Sprintf("sideBySide=%v render=%q", m.patchViewOptions.SideBySide,
			m.render(),
		),
	)
	testutil.RequireEqual(t, strings.Contains(m.render(), "│"), true,
		fmt.Sprintf("sideBySide=%v render=%q", m.patchViewOptions.SideBySide,
			m.render(),
		),
	)
}

func TestNewKeepsSavedSideBySideInactiveForNarrowInitialSize(t *testing.T) {
	m := newModel(singleHunkPatch(), nil, nil, initialLayout{
		SideBySide: true,
		size:       size{Width: 80, Height: 30},
	})
	testutil.RequireEqual(t, m.patchViewOptions.SideBySide, true,
		fmt.Sprintf("sideBySide=%v render=%q", m.patchViewOptions.SideBySide,
			m.render(),
		),
	)
	testutil.RequireEqual(t, !strings.Contains(m.render(), "│"), true,
		fmt.Sprintf("sideBySide=%v render=%q", m.patchViewOptions.SideBySide,
			m.render(),
		),
	)

	m = applyTestMessage(t, m, tea.WindowSizeMsg{Width: 120, Height: 30})
	testutil.RequireEqual(t, m.patchViewOptions.SideBySide, true,
		fmt.Sprintf("sideBySide=%v render=%q", m.patchViewOptions.SideBySide,
			m.render(),
		),
	)
	testutil.RequireEqual(t, strings.Contains(m.render(), "│"), true,
		fmt.Sprintf("sideBySide=%v render=%q", m.patchViewOptions.SideBySide,
			m.render(),
		),
	)
}

func TestSideBySideToggleStillSavesPreference(t *testing.T) {
	var saved []bool
	m := newModel(singleHunkPatch(), nil, nil, initialLayout{
		SaveSideBySide: func(enabled bool) error {
			saved = append(saved, enabled)
			return nil
		},
		size: size{Width: 120, Height: 30},
	})

	m = applyTestMessage(t, m, textKeyMessage("t"))
	applyTestMessage(t, m, textKeyMessage("t"))
	testutil.RequireEqual(t, slices.Equal(saved, []bool{true, false}), true,
		fmt.Sprintf("saved=%v", saved),
	)
}

func TestNarrowLayoutToggleSavesPreferenceWithoutApplicationGeometryRules(
	t *testing.T,
) {
	var saved []bool
	m := newModel(singleHunkPatch(), nil, nil, initialLayout{
		size: size{Width: 80, Height: 30},
		SaveSideBySide: func(enabled bool) error {
			saved = append(saved, enabled)
			return nil
		},
	})
	m = applyTestMessage(t, m, textKeyMessage("t"))
	testutil.RequireEqual(t, m.err, nil,
		fmt.Sprintf("narrow toggle: saved=%v err=%v render=%q", saved, m.err,
			m.render(),
		),
	)
	testutil.RequireEqual(t, slices.Equal(saved, []bool{true}), true,
		fmt.Sprintf("narrow toggle: saved=%v err=%v render=%q", saved, m.err,
			m.render(),
		),
	)
	testutil.RequireEqual(t, !strings.Contains(m.render(), " │ "), true,
		fmt.Sprintf("narrow toggle: saved=%v err=%v render=%q", saved, m.err,
			m.render(),
		),
	)

	m = applyTestMessage(t, m, tea.WindowSizeMsg{Width: 120, Height: 30})
	testutil.RequireEqual(t, strings.Contains(m.render(), " │ "), true,
		"widening did not activate preferred layout",
	)
}
