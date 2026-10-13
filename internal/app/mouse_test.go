package app

import (
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
		before := m.renderActiveScreen()
		m = updateModel(
			t,
			m,
			tea.MouseClickMsg{X: 0, Y: 0, Button: tea.MouseLeft},
		)
		m = updateModel(
			t,
			m,
			tea.MouseClickMsg{X: 0, Y: m.height - 2, Button: tea.MouseLeft},
		)
		testutil.RequireEqual(t, m.mode, mode, "%s",
			"header/footer click triggered an action",
		)
		testutil.RequireEqual(t, !m.quitting, true, "%s",
			"header/footer click triggered an action",
		)
		testutil.RequireEqual(t, m.renderActiveScreen(), before, "%s",
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
