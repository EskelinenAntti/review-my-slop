package app

import (
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestMouseWheelScrollsContentWithoutMovingFocus(t *testing.T) {
	m := testModel(longModelPatch(), nil, nil)
	m = updateModel(t, m, tea.WindowSizeMsg{Width: 37, Height: 9})
	m = updateModel(t, m, textKey("v"))
	m = updateModel(t, m, textKey("j"))
	_, focus, _ := m.diffView.Current()
	_, selected, _ := m.diffView.Selected()
	initial := m.render()
	for _, button := range []tea.MouseButton{
		tea.MouseWheelDown,
		tea.MouseWheelRight,
	} {
		m = updateModel(t, m, tea.MouseWheelMsg{Button: button})
	}
	testutil.RequireEqual(t, m.render() != initial, true,
		"wheel did not scroll content",
	)
	testutil.RequireEqual(t, strings.Contains(m.render(), "%)"), true,
		"wheel did not scroll content",
	)

	_, got, _ := m.diffView.Current()
	_, gotSelected, _ := m.diffView.Selected()
	testutil.RequireEqual(t, got, focus, "wheel changed focus or selection")
	testutil.RequireEqual(t, slices.Equal(selected, gotSelected), true,
		"wheel changed focus or selection",
	)

	for _, button := range []tea.MouseButton{
		tea.MouseWheelUp,
		tea.MouseWheelLeft,
	} {
		m = updateModel(t, m, tea.MouseWheelMsg{Button: button})
	}
	testutil.RequireEqual(t, m.render(), initial,
		"reverse wheel did not restore content",
	)

	for _, mode := range []mode{modeComments, modeHelp} {
		m.mode = mode
		before := m.diffView.Render(nil)
		m = updateModel(t, m, tea.MouseWheelMsg{Button: tea.MouseWheelDown})
		testutil.RequireEqual(t, m.diffView.Render(nil), before,
			"wheel scrolled diff behind another screen",
		)
	}
}
