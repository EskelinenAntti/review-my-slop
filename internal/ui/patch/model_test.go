package patch

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	corepatch "github.com/eskelinenantti/review-my-slop/internal/patch"
)

func testPatch() corepatch.Patch {
	return corepatch.Patch{Repository: "/repo", Fingerprint: "initial", Files: []corepatch.File{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", OldSource: "package main\nold()\nkeep()\n", NewSource: "package main\nnew()\nkeep()\nmore()\n", Hunks: []corepatch.Hunk{
		{Header: "@@ -1,3 +1,3 @@", Lines: []corepatch.Line{{Kind: corepatch.Context, Text: "package main", OldNumber: 1, NewNumber: 1}, {Kind: corepatch.Deletion, Text: "old()", OldNumber: 2}, {Kind: corepatch.Addition, Text: "new()", NewNumber: 2}, {Kind: corepatch.Context, Text: "keep()", OldNumber: 3, NewNumber: 3}}},
		{Header: "@@ -3,1 +3,2 @@", Lines: []corepatch.Line{{Kind: corepatch.Context, Text: "keep()", OldNumber: 3, NewNumber: 3}, {Kind: corepatch.Addition, Text: "more()", NewNumber: 4}}},
	}}}}
}

func modelForTest(width int, deps Dependencies) *Model {
	return New(Initial{Patch: testPatch(), DefaultBranch: "main", SideBySide: true, Width: width, Height: 12}, deps)
}

func sendKey(m *Model, name string) tea.Cmd {
	if len(name) == 1 {
		return m.Update(tea.KeyPressMsg(tea.Key{Text: name, Code: rune(name[0])}))
	}
	return m.Update(tea.KeyPressMsg(tea.Key{Code: map[string]rune{"tab": tea.KeyTab, "enter": tea.KeyEnter, "esc": tea.KeyEsc, "backspace": tea.KeyBackspace}[name]}))
}

func activeText(m *Model) string {
	snapshot := m.nav.Snapshot()
	if snapshot.Cursor == nil {
		return ""
	}
	pos, ok := m.doc.Position(*snapshot.Cursor)
	if !ok {
		return ""
	}
	return m.patch.Files[pos.File].Hunks[pos.Hunk].Lines[pos.Line].Text
}

func TestSavedSideBySidePreferenceSurvivesNarrowResize(t *testing.T) {
	m := modelForTest(120, Dependencies{})
	if !strings.Contains(ansi.Strip(m.Render()), "│") {
		t.Fatal("wide screen did not render split panes")
	}
	m.Resize(80, 12)
	if !m.sideBySide || strings.Contains(ansi.Strip(m.Render()), "│") {
		t.Fatal("narrow screen lost saved preference or stayed split")
	}
	m.Resize(120, 12)
	if !strings.Contains(ansi.Strip(m.Render()), "│") {
		t.Fatal("split panes did not return after widening")
	}
}

func TestToggleLayoutPersistsPreferenceAndRejectsNarrowEnable(t *testing.T) {
	var saved []bool
	m := New(Initial{Patch: testPatch(), Width: 80, Height: 12}, Dependencies{SaveLayout: func(enabled bool) error { saved = append(saved, enabled); return nil }})
	sendKey(m, "t")
	if m.err == nil || len(saved) != 0 {
		t.Fatalf("narrow toggle err=%v saves=%v", m.err, saved)
	}
	m.Resize(120, 12)
	sendKey(m, "t")
	sendKey(m, "t")
	if fmt.Sprint(saved) != "[true false]" {
		t.Fatalf("saved=%v", saved)
	}
}

func TestRenderHeaderAndFooterKeepReviewAppearance(t *testing.T) {
	m := modelForTest(80, Dependencies{})
	lines := strings.Split(ansi.Strip(m.Render()), "\n")
	if !strings.Contains(lines[0], "review-my-slop  +2-1") || !strings.Contains(lines[len(lines)-2], "local changes") {
		t.Fatalf("header/footer = %q / %q", lines[0], lines[len(lines)-2])
	}
	if !strings.Contains(m.Render(), "old()") || !strings.Contains(m.Render(), "new()") {
		t.Fatal("render omitted changed lines")
	}
}

func TestSearchIncrementalRepeatCancelAndFileName(t *testing.T) {
	m := modelForTest(120, Dependencies{})
	sendKey(m, "/")
	for _, r := range "keep" {
		sendKey(m, string(r))
	}
	first := m.nav.Snapshot().Cursor
	if m.mode != modeSearch || activeText(m) != "keep()" {
		t.Fatalf("search text %q mode %d", activeText(m), m.mode)
	}
	sendKey(m, "enter")
	sendKey(m, "n")
	if m.nav.Snapshot().Cursor == nil || first == nil || *m.nav.Snapshot().Cursor == *first || activeText(m) != "keep()" {
		t.Fatal("next search did not wrap to second match")
	}
	sendKey(m, "N")
	if m.nav.Snapshot().Cursor == nil || first == nil || *m.nav.Snapshot().Cursor != *first {
		t.Fatal("previous search did not return to first match")
	}
	sendKey(m, "/")
	for _, r := range "missing" {
		sendKey(m, string(r))
	}
	if !m.searchMiss {
		t.Fatal("missing query did not show miss state")
	}
	sendKey(m, "esc")
	if m.nav.Snapshot().Cursor == nil || first == nil || *m.nav.Snapshot().Cursor != *first {
		t.Fatal("cancel did not restore the search origin")
	}

	sendKey(m, "/")
	for _, r := range "main.go" {
		sendKey(m, string(r))
	}
	pos, ok := m.doc.Position(*m.nav.Snapshot().Cursor)
	if !ok || m.patch.Files[pos.File].DisplayPath != "main.go" {
		t.Fatal("search did not match file name")
	}
	for range len("main.go") {
		sendKey(m, "backspace")
	}
	if m.nav.Snapshot().Cursor == nil || *m.nav.Snapshot().Cursor != *m.searchFrom {
		t.Fatal("empty query did not restore origin")
	}
}

func TestRefreshRejectsOlderSameBranchAndObsoleteBranchResults(t *testing.T) {
	m := modelForTest(120, Dependencies{Load: func(branch string) (corepatch.Patch, error) { p := testPatch(); p.Fingerprint = branch; return p, nil }})
	m.showDefault = true
	older, newer := m.requestRefresh(), m.requestRefresh()
	newResult := newer()
	m.Update(newResult)
	if m.patch.Fingerprint != "main" {
		t.Fatalf("new result fingerprint = %q", m.patch.Fingerprint)
	}
	m.Update(older())
	if m.patch.Fingerprint != "main" {
		t.Fatal("older same-branch result overwrote newer result")
	}

	branchResult := m.requestRefresh()
	m.showDefault = false
	m.Update(branchResult())
	if m.patch.Fingerprint != "main" {
		t.Fatal("obsolete branch result was applied")
	}
}

func TestRefreshAndSourceEditorCompletionsReloadCurrentPatch(t *testing.T) {
	loads := 0
	m := modelForTest(120, Dependencies{Load: func(string) (corepatch.Patch, error) {
		loads++
		p := testPatch()
		p.Fingerprint = fmt.Sprintf("load-%d", loads)
		return p, nil
	}})
	cmd := m.Update(tea.FocusMsg{})
	if cmd == nil {
		t.Fatal("focus did not request refresh")
	}
	m.Update(cmd())
	cmd = sendKey(m, "R")
	if cmd == nil {
		t.Fatal("R did not request refresh")
	}
	m.Update(cmd())
	cmd = m.Update(sourceEditorResult{})
	if cmd == nil {
		t.Fatal("successful editor completion did not request refresh")
	}
	m.Update(cmd())
	if loads != 3 || m.patch.Fingerprint != "load-3" {
		t.Fatalf("loads=%d fingerprint=%q", loads, m.patch.Fingerprint)
	}
}

func TestCommentSavedCancelsSelectionAndKeysEmitRoutingEvents(t *testing.T) {
	m := modelForTest(120, Dependencies{})
	sendKey(m, "v")
	if m.nav.Snapshot().Selection == nil {
		t.Fatal("v did not begin selection")
	}
	m.Update(CommentSaved{})
	if m.nav.Snapshot().Selection != nil {
		t.Fatal("CommentSaved did not cancel selection")
	}
	for key, want := range map[string]any{"c": CommentRequested{}, "C": CommentsRequested{}, "?": HelpRequested{}, "q": QuitRequested{}} {
		cmd := sendKey(m, key)
		if cmd == nil {
			t.Fatalf("%q emitted no event", key)
		}
		got := cmd()
		switch want.(type) {
		case CommentRequested:
			if _, ok := got.(CommentRequested); !ok {
				t.Fatalf("%q result = %T", key, got)
			}
		case CommentsRequested:
			if _, ok := got.(CommentsRequested); !ok {
				t.Fatalf("%q result = %T", key, got)
			}
		case HelpRequested:
			if _, ok := got.(HelpRequested); !ok {
				t.Fatalf("%q result = %T", key, got)
			}
		case QuitRequested:
			if _, ok := got.(QuitRequested); !ok {
				t.Fatalf("%q result = %T", key, got)
			}
		}
	}
}
