package app

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/eskelinenantti/review-my-slop/internal/comments"
)

func TestMouseDragCreatesCommentSelection(t *testing.T) {
	t.Setenv("EDITOR", "true")
	for _, release := range []tea.MouseButton{tea.MouseLeft, tea.MouseNone} {
		m := testModel(coveragePatch(), nil, nil)
		m = updateModel(t, m, tea.MouseClickMsg{X: 20, Y: 4, Button: tea.MouseLeft})
		if lineText(m) != "old()" {
			t.Fatal("click did not focus deletion")
		}
		m = updateModel(t, m, tea.MouseMotionMsg{X: 20, Y: 5, Button: tea.MouseLeft})
		m = updateModel(t, m, tea.MouseReleaseMsg{X: 20, Y: 6, Button: release})
		_, lines, ok := m.diffView.Selected()
		if !ok || len(lines) != 3 || lineText(m) != "keep()" {
			t.Fatalf("released selection = %#v", lines)
		}
		m = updateModel(t, m, tea.MouseMotionMsg{X: 20, Y: 3, Button: tea.MouseLeft})
		if lineText(m) != "keep()" {
			t.Fatal("motion after release changed focus")
		}
		m = updateModel(t, m, textKey("c"))
		if len(m.edit.anchor.QuotedLines) != 3 || m.edit.anchor.QuotedLines[0] != "-old()" {
			t.Fatalf("comment anchor = %#v", m.edit.anchor)
		}
	}
}

func TestMouseClickCommentsAndIgnoreInformationalUI(t *testing.T) {
	m := testModel(coveragePatch(), []comments.Comment{{ID: "one"}, {ID: "two"}}, nil)
	m = updateModel(t, m, textKey("C"))
	m = updateModel(t, m, tea.MouseClickMsg{X: 10, Y: 2, Button: tea.MouseLeft})
	selected, _ := m.commentView.Selected()
	if selected.ID != "two" {
		t.Fatal("click did not focus comment")
	}
	for _, mode := range []mode{modeBrowse, modeComments, modeHelp, modeSearch} {
		m.mode = mode
		before := m.render()
		m = updateModel(t, m, tea.MouseClickMsg{X: 0, Y: 0, Button: tea.MouseLeft})
		m = updateModel(t, m, tea.MouseClickMsg{X: 0, Y: m.height - 2, Button: tea.MouseLeft})
		if m.mode != mode || m.quitting || m.render() != before {
			t.Fatal("header/footer click triggered an action")
		}
	}
}

func TestMouseIgnoresOtherButtonsAndKeyboardCancelsDrag(t *testing.T) {
	m := testModel(coveragePatch(), nil, nil)
	m = updateModel(t, m, tea.MouseClickMsg{X: 10, Y: 5, Button: tea.MouseRight})
	if lineText(m) != "package main" {
		t.Fatal("right click moved focus")
	}
	m = updateModel(t, m, tea.MouseMotionMsg{X: 10, Y: 5, Button: tea.MouseLeft})
	if lineText(m) != "package main" {
		t.Fatal("motion without a press moved focus")
	}
	m = updateModel(t, m, tea.MouseClickMsg{X: 10, Y: 3, Button: tea.MouseLeft})
	m = updateModel(t, m, textKey("j"))
	m = updateModel(t, m, tea.MouseMotionMsg{X: 10, Y: 5, Button: tea.MouseLeft})
	if lineText(m) != "old()" {
		t.Fatal("keyboard action did not cancel drag")
	}
}
