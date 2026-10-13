package app

import (
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestMouseWheelScrollsContentWithoutMovingFocus(t *testing.T) {
	m := newTestModel(longContextPatch(), nil, nil)
	m = applyTestMessage(t, m, tea.WindowSizeMsg{Width: 37, Height: 9})
	m = applyTestMessage(t, m, textKeyMessage("v"))
	m = applyTestMessage(t, m, textKeyMessage("j"))
	_, focus, _ := m.patchView.Current()
	_, selected, _ := m.patchView.Selected()
	initial := m.render()
	for _, button := range []tea.MouseButton{
		tea.MouseWheelDown,
		tea.MouseWheelRight,
	} {
		m = applyTestMessage(t, m, tea.MouseWheelMsg{Button: button})
	}
	testutil.RequireEqual(t, m.render() != initial, true,
		"wheel did not scroll content",
	)
	testutil.RequireEqual(t, strings.Contains(m.render(), "%)"), true,
		"wheel did not scroll content",
	)

	_, got, _ := m.patchView.Current()
	_, gotSelected, _ := m.patchView.Selected()
	testutil.RequireEqual(t, got, focus, "wheel changed focus or selection")
	testutil.RequireEqual(t, slices.Equal(selected, gotSelected), true,
		"wheel changed focus or selection",
	)

	for _, button := range []tea.MouseButton{
		tea.MouseWheelUp,
		tea.MouseWheelLeft,
	} {
		m = applyTestMessage(t, m, tea.MouseWheelMsg{Button: button})
	}
	testutil.RequireEqual(t, m.render(), initial,
		"reverse wheel did not restore content",
	)

	for _, mode := range []mode{modeComments, modeHelp} {
		m.mode = mode
		before := m.patchView.Render(nil)
		m = applyTestMessage(
			t, m, tea.MouseWheelMsg{Button: tea.MouseWheelDown},
		)
		testutil.RequireEqual(t, m.patchView.Render(nil), before,
			"wheel scrolled diff behind another screen",
		)
	}
}
