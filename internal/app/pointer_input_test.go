package app

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	feedback "github.com/eskelinenantti/review-my-slop/internal/comments"
)

func TestAppMouseDragCreatesCommentSelection(t *testing.T) {
	t.Setenv("EDITOR", "true")
	for _, release := range []tea.MouseButton{tea.MouseLeft, tea.MouseNone} {
		m := newTestBasicAppModel()
		m = updateTestAppModel(t, m, tea.MouseClickMsg{
			X:      20,
			Y:      4,
			Button: tea.MouseLeft,
		})
		requireEqual(
			t,
			currentTestDiffLineText(m),
			"old()",
			"%s",
			"click did not focus deletion",
		)

		m = updateTestAppModel(t, m, tea.MouseMotionMsg{
			X:      20,
			Y:      5,
			Button: tea.MouseLeft,
		})
		m = updateTestAppModel(t, m, tea.MouseReleaseMsg{
			X:      20,
			Y:      6,
			Button: release,
		})
		_, lines, ok := m.diffView.Selected()
		requireEqual(t, ok, true, "released selection = %#v", lines)
		requireEqual(t, len(lines), 3, "released selection = %#v", lines)
		requireEqual(
			t,
			currentTestDiffLineText(m),
			"keep()",
			"released selection = %#v",
			lines,
		)

		m = updateTestAppModel(t, m, tea.MouseMotionMsg{
			X:      20,
			Y:      3,
			Button: tea.MouseLeft,
		})
		requireEqual(
			t,
			currentTestDiffLineText(m),
			"keep()",
			"%s",
			"motion after release changed focus",
		)

		m = pressTestKey(t, m, "c")
		requireEqual(t, len(m.edit.anchor.QuotedLines), 3,
			"comment anchor = %#v", m.edit.anchor,
		)
		requireEqual(t, m.edit.anchor.QuotedLines[0], "-old()",
			"comment anchor = %#v", m.edit.anchor,
		)
	}
}

func TestAppMouseClickCommentsAndIgnoreInformationalUI(t *testing.T) {
	items := []feedback.Comment{{ID: "one"}, {ID: "two"}}
	m := newTestBasicAppModel()
	m.comments = commentState{items: items}
	m.commentView = newTestCommentScreen(items)
	m.commentView.Resize(m.width, m.height)
	m = pressTestKey(t, m, "C")
	m = updateTestAppModel(t, m, tea.MouseClickMsg{
		X:      10,
		Y:      2,
		Button: tea.MouseLeft,
	})
	selected, _ := m.commentView.Selected()
	requireEqual(t, selected.ID, "two", "%s", "click did not focus comment")

	for _, inlineLocalMode1 := range []mode{
		modeBrowse,
		modeComments,
		modeHelp,
		modeSearch,
	} {
		m.mode = inlineLocalMode1
		before := renderAppWorkflowMode(&m)
		m = updateTestAppModel(t, m, tea.MouseClickMsg{
			X:      0,
			Y:      0,
			Button: tea.MouseLeft,
		})
		m = updateTestAppModel(t, m,
			tea.MouseClickMsg{X: 0, Y: m.height - 2, Button: tea.MouseLeft},
		)
		requireEqual(t, m.mode, inlineLocalMode1, "%s",
			"header/footer click triggered an action",
		)
		requireEqual(t, !m.quitting, true, "%s",
			"header/footer click triggered an action",
		)
		requireEqual(
			t,
			renderAppWorkflowMode(&m),
			before,
			"%s",
			"header/footer click triggered an action",
		)
	}
}

func TestAppMouseIgnoresOtherButtonsAndKeyboardCancelsDrag(t *testing.T) {
	m := newTestBasicAppModel()
	m = updateTestAppModel(t, m, tea.MouseClickMsg{
		X: 10, Y: 5, Button: tea.MouseRight,
	})
	requireEqual(
		t,
		currentTestDiffLineText(m),
		"package main",
		"%s",
		"right click moved focus",
	)

	m = updateTestAppModel(t, m, tea.MouseMotionMsg{
		X: 10, Y: 5, Button: tea.MouseLeft,
	})
	requireEqual(
		t,
		currentTestDiffLineText(m),
		"package main",
		"%s",
		"motion without a press moved focus",
	)

	m = updateTestAppModel(t, m, tea.MouseClickMsg{
		X: 10, Y: 3, Button: tea.MouseLeft,
	})
	m = pressTestKey(t, m, "j")
	m = updateTestAppModel(t, m, tea.MouseMotionMsg{
		X: 10, Y: 5, Button: tea.MouseLeft,
	})
	requireEqual(
		t,
		currentTestDiffLineText(m),
		"old()",
		"%s",
		"keyboard action did not cancel drag",
	)
}
