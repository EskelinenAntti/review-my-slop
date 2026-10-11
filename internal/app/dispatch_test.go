package app

import (
	"image/color"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/eskelinenantti/review-my-slop/internal/comments"
)

func TestHelpKeysCloseHelpAndQuitOnlyFromBrowse(t *testing.T) {
	for _, key := range []string{"esc", "?", "q"} {
		m := testModel(modelPatch(), nil, nil)
		m = updateModel(t, m, textKey("?"))
		if m.mode != modeHelp {
			t.Fatal("help did not open")
		}
		m = updateModel(t, m, textKey("j"))
		if m.mode != modeHelp {
			t.Fatal("unrelated key closed help")
		}
		m = updateModel(t, m, textKey(key))
		if m.mode != modeBrowse || m.quitting {
			t.Fatalf("key=%q mode=%v quitting=%v", key, m.mode, m.quitting)
		}
		m = updateModel(t, m, textKey("q"))
		if !m.quitting {
			t.Fatal("browse quit ignored")
		}
	}
}

func TestCommentKeysMoveListFocus(t *testing.T) {
	items := []comments.Comment{{ID: "one", Body: "one"}, {ID: "two", Body: "two"}}
	m := testModel(modelPatch(), items, nil)
	m = updateModel(t, m, textKey("C"))
	m = updateModel(t, m, textKey("j"))
	selected, _ := m.commentView.Selected()
	if selected.ID != "two" {
		t.Fatalf("selected=%+v", selected)
	}
	m = updateModel(t, m, textKey("k"))
	selected, _ = m.commentView.Selected()
	if selected.ID != "one" {
		t.Fatalf("selected=%+v", selected)
	}
}

func TestBackgroundColorUpdatesPresentation(t *testing.T) {
	m := testModel(modelPatch(), nil, nil)
	for _, tc := range []struct {
		color color.Color
		dark  bool
	}{
		{color.White, false}, {color.White, false}, {color.Black, true},
	} {
		m = updateModel(t, m, tea.BackgroundColorMsg{Color: tc.color})
		if m.diffOptions.Dark != tc.dark {
			t.Fatalf("dark=%v, want %v", m.diffOptions.Dark, tc.dark)
		}
	}
}
