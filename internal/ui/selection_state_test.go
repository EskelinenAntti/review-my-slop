package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

func TestBrowseMovementKeepsSingleLineSelection(t *testing.T) {
	m := testModel(longModelPatch(), nil, nil)
	for _, key := range []string{"j", "G", "g", "g", "ctrl+d", "ctrl+u", "k"} {
		m = updateModel(t, m, textKey(key))
		if m.review.Selection == nil || m.review.Selection.First != m.review.Selection.Last || m.review.Extending {
			t.Fatalf("after %q: %#v", key, m.review.State)
		}
	}
	if strings.Contains(ansi.Strip(m.renderStatus()), "visual selection") {
		t.Fatal("normal cursor displays visual-selection status")
	}
}

func TestVisualSelectionCanCollapseWithoutExitingExtension(t *testing.T) {
	for _, cancel := range []tea.KeyPressMsg{textKey("v"), specialKey(tea.KeyEsc)} {
		m := testModel(coveragePatch(), nil, nil)
		before := m.render()
		m = updateModel(t, m, textKey("v"))
		if !m.review.Extending || !strings.Contains(ansi.Strip(m.renderStatus()), "visual selection") {
			t.Fatal("v did not activate single-line visual selection")
		}
		if got := m.review.view.Render(m.review.Viewport, m.review.Selection); got != strings.Join(strings.Split(before, "\n")[1:m.height-2], "\n") {
			t.Fatal("single-line visual selection changed code rendering")
		}
		m = updateModel(t, m, textKey("j"))
		m = updateModel(t, m, textKey("k"))
		if !m.review.Extending || m.review.Selection.First != m.review.Selection.Last {
			t.Fatal("returning to anchor exited extension")
		}
		m = updateModel(t, m, textKey("j"))
		active := m.review.Cursor()
		m = updateModel(t, m, cancel)
		if m.review.Extending || m.review.Selection == nil || m.review.Selection.First != active || m.review.Selection.Last != active {
			t.Fatalf("cancel %q did not collapse at active endpoint: %#v", cancel.String(), m.review.State)
		}
	}
}

func TestVisualJumpsAndSearchUpdateCommentRange(t *testing.T) {
	for _, tc := range []struct {
		name  string
		keys  []string
		term  string
		want  string
		count int
	}{
		{name: "last", keys: []string{"G"}, want: "line 29", count: 20},
		{name: "first", keys: []string{"g", "g"}, want: "line 0", count: 11},
		{name: "next match", keys: []string{"n"}, term: "line 20 ", want: "line 20", count: 11},
		{name: "previous match", keys: []string{"N"}, term: "line 5 ", want: "line 5", count: 6},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := testModel(longModelPatch(), nil, nil)
			m.setCursor(findLine(t, m, "line 10 "+strings.Repeat("x", 80)))
			anchor := m.review.Cursor()
			m.search.term = tc.term
			m = updateModel(t, m, textKey("v"))
			for _, key := range tc.keys {
				m = updateModel(t, m, textKey(key))
			}
			if m.review.Selection.First != anchor || !m.review.Extending || !strings.HasPrefix(lineText(m), tc.want+" ") {
				t.Fatalf("selection=%#v line=%q", m.review.State, lineText(m))
			}
			comment, err := m.review.view.Anchor(*m.review.Selection)
			if err != nil || len(comment.QuotedLines) != tc.count {
				t.Fatalf("comment range=%#v err=%v", comment, err)
			}
			m = updateModel(t, m, textKey("j"))
			if m.review.Selection.First != anchor {
				t.Fatal("movement after jump lost selection anchor")
			}
		})
	}
}

func TestVisualMovementRejectsOtherHunksAndFilesAtomically(t *testing.T) {
	for _, boundary := range []string{"hunk", "file"} {
		for _, key := range []string{"j", "ctrl+d", "G", "n"} {
			t.Run(boundary+"/"+key, func(t *testing.T) {
				p := coveragePatch()
				if boundary == "file" {
					other := p.Files[0]
					other.DisplayPath, other.OldPath, other.NewPath = "other.go", "other.go", "other.go"
					p.Files[0].Hunks = p.Files[0].Hunks[:1]
					other.Hunks = other.Hunks[1:]
					p.Files = append(p.Files, other)
				}
				m := testModel(p, nil, nil)
				m.setCursor(findLine(t, m, "new()"))
				m = updateModel(t, m, textKey("v"))
				m = updateModel(t, m, textKey("j"))
				m.search.term = "more()"
				before := m.review.State
				selection := *before.Selection
				m = updateModel(t, m, textKey(key))
				if *m.review.Selection != selection || m.review.Viewport != before.Viewport || !m.review.Extending {
					t.Fatalf("rejected movement changed state: before=%#v after=%#v", before, m.review.State)
				}
			})
		}
	}
	m := testModel(coveragePatch(), nil, nil)
	m.setCursor(findLine(t, m, "more()"))
	m = updateModel(t, m, textKey("v"))
	before := *m.review.Selection
	m = updateModel(t, m, textKey("g"))
	m = updateModel(t, m, textKey("g"))
	if *m.review.Selection != before {
		t.Fatal("gg crossed a hunk boundary")
	}
}

func TestVisualPaneSwitchAndLayoutPreserveActiveEndpoint(t *testing.T) {
	m := testModel(coveragePatch(), nil, nil)
	m.width = 120
	m.setSideBySide(true)
	m.setCursor(findLine(t, m, "new()"))
	m = updateModel(t, m, textKey("v"))
	m = updateModel(t, m, textKey("j"))
	m.switchPane(Left)
	first, _ := m.review.view.Line(m.review.Selection.First)
	if first.Text != "old()" || lineText(m) != "keep()" || m.review.Selection.Last.Pane != Left || !m.review.Extending {
		t.Fatalf("pane switch changed selection: %#v", m.review.State)
	}
	for _, width := range []int{80, 120} {
		m = updateModel(t, m, tea.WindowSizeMsg{Width: width, Height: 20})
		wantLines := 2
		if width == 80 {
			wantLines = 3
		}
		if !m.review.Extending || lineText(m) != "keep()" || len(m.review.view.Lines(*m.review.Selection)) != wantLines {
			t.Fatalf("resize to %d lost selection: %#v", width, m.review.State)
		}
	}
}

func TestEmptyViewSelectionLifecycle(t *testing.T) {
	m := testModel(patch.Patch{}, nil, nil)
	for _, key := range []string{"v", "j", "G", "g", "g", "ctrl+d", "esc"} {
		m = updateModel(t, m, textKey(key))
		if m.review.Selection != nil || m.review.Extending {
			t.Fatalf("empty view created selection after %q", key)
		}
	}
	if _, err := m.beginComment(); err == nil || err.Error() != "select code lines before commenting" {
		t.Fatalf("empty-view comment error=%v", err)
	}
	m.rebuildView(coveragePatch())
	first, _ := m.review.view.First()
	if m.review.Selection == nil || m.review.Selection.First != first || m.review.Cursor() != first || m.review.Extending {
		t.Fatalf("nonempty refresh did not create cursor: %#v", m.review.State)
	}
	m = updateModel(t, m, textKey("v"))
	m.rebuildView(patch.Patch{})
	if m.review.Selection != nil || m.review.Extending {
		t.Fatal("empty refresh retained selection")
	}
}

func TestSelectionTransitionsDoNotMutatePreviousModel(t *testing.T) {
	m := testModel(coveragePatch(), nil, nil)
	m = updateModel(t, m, textKey("v"))
	before := *m.review.Selection
	next := updateModel(t, m, textKey("j"))
	if *m.review.Selection != before || next.review.Selection.Last == before.Last {
		t.Fatal("selection movement mutated previous model or failed to move")
	}
}
