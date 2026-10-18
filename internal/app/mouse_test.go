package app

import (
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/eskelinenantti/review-my-slop/internal/comments"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestMouseDragCreatesCommentSelection(t *testing.T) {
	t.Setenv("EDITOR", "true")
	for _, release := range []tea.MouseButton{tea.MouseLeft, tea.MouseNone} {
		m := testModel(coveragePatch(), nil, nil)
		m = updateModel(
			t,
			m,
			tea.MouseClickMsg{X: 20, Y: 4, Button: tea.MouseLeft},
		)
		testutil.RequireEqual(t, lineText(m), "old()", "%s",
			"click did not focus deletion",
		)

		m = updateModel(
			t,
			m,
			tea.MouseMotionMsg{X: 20, Y: 5, Button: tea.MouseLeft},
		)
		m = updateModel(t, m, tea.MouseReleaseMsg{X: 20, Y: 6, Button: release})
		_, lines, ok := m.diffView.Selected()
		testutil.RequireEqual(t, ok, true, "released selection = %#v", lines)
		testutil.RequireEqual(t, len(lines), 3, "released selection = %#v",
			lines,
		)
		testutil.RequireEqual(t, lineText(m), "keep()",
			"released selection = %#v", lines,
		)

		m = updateModel(
			t,
			m,
			tea.MouseMotionMsg{X: 20, Y: 3, Button: tea.MouseLeft},
		)
		testutil.RequireEqual(t, lineText(m), "keep()", "%s",
			"motion after release changed focus",
		)

		m = updateModel(t, m, textKey("c"))
		testutil.RequireEqual(t, len(m.edit.anchor.QuotedLines), 3,
			"comment anchor = %#v", m.edit.anchor,
		)
		testutil.RequireEqual(t, m.edit.anchor.QuotedLines[0], "-old()",
			"comment anchor = %#v", m.edit.anchor,
		)
	}
}

func TestMouseClickCommentsAndIgnoreInformationalUI(t *testing.T) {
	m := testModel(
		coveragePatch(),
		[]comments.Comment{{ID: "one"}, {ID: "two"}},
		nil,
	)
	m = updateModel(t, m, textKey("C"))
	m = updateModel(t, m, tea.MouseClickMsg{X: 10, Y: 2, Button: tea.MouseLeft})
	selected, _ := m.commentView.Selected()
	testutil.RequireEqual(t, selected.ID, "two", "%s",
		"click did not focus comment",
	)

	for _, mode := range []mode{
		modeBrowse,
		modeComments,
		modeHelp,
		modeSearch,
	} {
		m.mode = mode
		before := m.render()
		m = updateModel(
			t,
			m,
			tea.MouseClickMsg{X: 0, Y: 0, Button: tea.MouseLeft},
		)
		m = updateModel(
			t,
			m,
			tea.MouseClickMsg{
				X: 0, Y: m.dimensions.height - 2, Button: tea.MouseLeft,
			},
		)
		testutil.RequireEqual(t, m.mode, mode, "%s",
			"header/footer click triggered an action",
		)
		testutil.RequireEqual(t, !m.quitting, true, "%s",
			"header/footer click triggered an action",
		)
		testutil.RequireEqual(t, m.render(), before, "%s",
			"header/footer click triggered an action",
		)
	}
}

func TestMouseIgnoresOtherButtonsAndKeyboardCancelsDrag(t *testing.T) {
	m := testModel(coveragePatch(), nil, nil)
	m = updateModel(
		t,
		m,
		tea.MouseClickMsg{X: 10, Y: 5, Button: tea.MouseRight},
	)
	testutil.RequireEqual(t, lineText(m), "package main", "%s",
		"right click moved focus",
	)

	m = updateModel(
		t,
		m,
		tea.MouseMotionMsg{X: 10, Y: 5, Button: tea.MouseLeft},
	)
	testutil.RequireEqual(t, lineText(m), "package main", "%s",
		"motion without a press moved focus",
	)

	m = updateModel(t, m, tea.MouseClickMsg{X: 10, Y: 3, Button: tea.MouseLeft})
	m = updateModel(t, m, textKey("j"))
	m = updateModel(
		t,
		m,
		tea.MouseMotionMsg{X: 10, Y: 5, Button: tea.MouseLeft},
	)
	testutil.RequireEqual(t, lineText(m), "old()", "%s",
		"keyboard action did not cancel drag",
	)
}

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
	testutil.RequireEqual(t, m.render() != initial, true, "%s",
		"wheel did not scroll content",
	)
	testutil.RequireEqual(t, strings.Contains(m.render(), "%)"), true, "%s",
		"wheel did not scroll content",
	)

	_, got, _ := m.diffView.Current()
	_, gotSelected, _ := m.diffView.Selected()
	testutil.RequireEqual(t, got, focus, "%s",
		"wheel changed focus or selection",
	)
	testutil.RequireEqual(t, slices.Equal(selected, gotSelected), true, "%s",
		"wheel changed focus or selection",
	)

	for _, button := range []tea.MouseButton{
		tea.MouseWheelUp,
		tea.MouseWheelLeft,
	} {
		m = updateModel(t, m, tea.MouseWheelMsg{Button: button})
	}
	testutil.RequireEqual(t, m.render(), initial, "%s",
		"reverse wheel did not restore content",
	)

	for _, mode := range []mode{modeComments, modeHelp} {
		m.mode = mode
		before := m.diffView.Render(nil)
		m = updateModel(t, m, tea.MouseWheelMsg{Button: tea.MouseWheelDown})
		testutil.RequireEqual(t, m.diffView.Render(nil), before, "%s",
			"wheel scrolled patch behind another screen",
		)
	}
}
