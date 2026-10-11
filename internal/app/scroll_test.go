package app

import (
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestMouseWheelScrollsContentWithoutMovingFocus(t *testing.T) {
	m := testModel(longModelPatch(), nil, nil)
	m = updateModel(
		t,
		m,
		tea.WindowSizeMsg{Width: narrowTestWidth, Height: 9},
	)
	m = updateModel(t, m, textKey(selectionKey))
	m = updateModel(t, m, textKey(keyJ))
	_, focus, _ := m.diffView.Current()
	_, selected, _ := m.diffView.Selected()
	initial := m.render()
	for _, button := range []tea.MouseButton{
		tea.MouseWheelDown,
		tea.MouseWheelRight,
	} {
		m = updateModel(t, m, tea.MouseWheelMsg{Button: button})
	}
	requireEqual(
		t,
		m.render() != initial && strings.Contains(m.render(), progressSuffix),
		true,
		valueFormat,
		"wheel did not scroll content",
	)
	_, got, _ := m.diffView.Current()
	_, gotSelected, _ := m.diffView.Selected()
	requireEqual(
		t,
		got == focus && slices.Equal(selected, gotSelected),
		true,
		valueFormat,
		"wheel changed focus or selection",
	)
	for _, button := range []tea.MouseButton{
		tea.MouseWheelUp,
		tea.MouseWheelLeft,
	} {
		m = updateModel(t, m, tea.MouseWheelMsg{Button: button})
	}
	requireEqual(
		t,
		m.render(), initial,
		valueFormat,
		"reverse wheel did not restore content",
	)
	for _, mode := range []mode{modeComments, modeHelp} {
		m.mode = mode
		before := m.diffView.Render(nil)
		m = updateModel(t, m, tea.MouseWheelMsg{Button: tea.MouseWheelDown})
		requireEqual(
			t,
			m.diffView.Render(nil), before,
			valueFormat,
			"wheel scrolled diff behind another screen",
		)
	}
}
