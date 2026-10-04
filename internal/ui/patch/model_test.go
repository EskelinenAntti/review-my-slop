package patch

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/eskelinenantti/review-my-slop/internal/layout"
	"github.com/eskelinenantti/review-my-slop/internal/navigation"
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

func sendCtrl(m *Model, letter rune) tea.Cmd {
	return m.Update(tea.KeyPressMsg(tea.Key{Code: letter, Mod: tea.ModCtrl}))
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
	rendered := ansi.Strip(m.Render())
	if !strings.Contains(rendered, "old()") || !strings.Contains(rendered, "new()") {
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
	loads := 0
	m := modelForTest(120, Dependencies{Load: func(branch string) (corepatch.Patch, error) {
		loads++
		p := testPatch()
		p.Fingerprint = fmt.Sprintf("%s-%d", branch, loads)
		return p, nil
	}})
	m.showDefault = true
	older, newer := m.requestRefresh(), m.requestRefresh()
	newResult := newer()
	m.Update(newResult)
	if m.patch.Fingerprint != "main-1" {
		t.Fatalf("new result fingerprint = %q", m.patch.Fingerprint)
	}
	m.Update(older())
	if m.patch.Fingerprint != "main-1" {
		t.Fatal("older same-branch result overwrote newer result")
	}

	branchResult := m.requestRefresh()
	m.showDefault = false
	m.Update(branchResult())
	if m.patch.Fingerprint != "main-1" {
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

func longPatch() corepatch.Patch {
	lines := make([]corepatch.Line, 40)
	for index := range lines {
		lines[index] = corepatch.Line{Kind: corepatch.Context, Text: fmt.Sprintf("line %d %s", index, strings.Repeat("x", 80)), OldNumber: corepatch.LineNumber(index + 1), NewNumber: corepatch.LineNumber(index + 1)}
	}
	return corepatch.Patch{Repository: "/repo", Fingerprint: "long", Files: []corepatch.File{{DisplayPath: "long.go", OldPath: "long.go", NewPath: "long.go", Hunks: []corepatch.Hunk{{Header: "@@ -1,40 +1,40 @@", Lines: lines}}}}}
}

func ctrlKey(letter rune) tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Code: letter, Mod: tea.ModCtrl})
}

func findCell(m *Model, text string) layout.Cell {
	for row := 0; row < m.doc.RowCount(); row++ {
		for _, pane := range []layout.Pane{layout.Right, layout.Left} {
			cell := layout.Cell{Row: row, Pane: pane}
			if !m.doc.Valid(cell) {
				continue
			}
			pos, _ := m.doc.Position(cell)
			if m.patch.Files[pos.File].Hunks[pos.Hunk].Lines[pos.Line].Text == text {
				return cell
			}
		}
	}
	return layout.Cell{Row: -1}
}

func TestVimSequencesAndPendingKeyConsumption(t *testing.T) {
	m := modelForTest(120, Dependencies{})
	sendKey(m, "G")
	last := m.nav.Snapshot().Cursor
	sendKey(m, "g")
	sendKey(m, "g")
	first := m.nav.Snapshot().Cursor
	if last == nil || first == nil || *first == *last || activeText(m) != "package main" {
		t.Fatalf("gg cursor=%v line=%q", first, activeText(m))
	}
	sendKey(m, "G")
	cursor := m.nav.Snapshot().Cursor
	sendKey(m, "g")
	sendKey(m, "h")
	sendKey(m, "g")
	if m.nav.Snapshot().Cursor == nil || cursor == nil || *m.nav.Snapshot().Cursor != *cursor || m.pendingKey != "g" {
		t.Fatal("pending g was not consumed by the next key")
	}
}

func TestZSequencesAlignCursorInViewport(t *testing.T) {
	m := New(Initial{Patch: longPatch(), Width: 80, Height: 9}, Dependencies{})
	m.nav.Jump(findCell(m, "line 20 "+strings.Repeat("x", 80)))
	for _, test := range []struct {
		key        string
		align      navigation.Alignment
		wantOffset int
	}{{"z", navigation.Middle, 2}, {"t", navigation.Top, 0}, {"b", navigation.Bottom, 4}} {
		sendKey(m, "z")
		sendKey(m, test.key)
		snapshot := m.nav.Snapshot()
		if snapshot.Cursor == nil {
			t.Fatal("alignment lost cursor")
		}
		if got := snapshot.Cursor.Row - snapshot.Viewport.Top; got != test.wantOffset {
			t.Errorf("z%s cursor offset=%d want=%d", test.key, got, test.wantOffset)
		}
	}
}

func TestCtrlWSwitchesPairedPanes(t *testing.T) {
	m := modelForTest(120, Dependencies{})
	m.nav.Jump(findCell(m, "new()"))
	sendCtrl(m, 'w')
	sendKey(m, "h")
	if got := activeText(m); got != "old()" {
		t.Fatalf("left pane line=%q", got)
	}
	sendCtrl(m, 'w')
	sendKey(m, "l")
	if got := activeText(m); got != "new()" {
		t.Fatalf("right pane line=%q", got)
	}
}

func TestHorizontalScrollStepAndResetKeys(t *testing.T) {
	m := New(Initial{Patch: longPatch(), Width: 80, Height: 12}, Dependencies{})
	sendKey(m, "l")
	if got := m.nav.Snapshot().Viewport.LeftColumn; got != horizontalScrollStep {
		t.Fatalf("right scroll=%d", got)
	}
	sendKey(m, "h")
	if got := m.nav.Snapshot().Viewport.LeftColumn; got != 0 {
		t.Fatalf("left scroll=%d", got)
	}
	sendKey(m, "$")
	if got := m.nav.Snapshot().Viewport.LeftColumn; got == 0 {
		t.Fatal("$ did not scroll to line end")
	}
	sendKey(m, "0")
	if got := m.nav.Snapshot().Viewport.LeftColumn; got != 0 {
		t.Fatalf("0 reset=%d", got)
	}
}

func TestSelectionCannotCrossHunk(t *testing.T) {
	m := modelForTest(80, Dependencies{})
	sendKey(m, "v")
	for range 10 {
		sendKey(m, "j")
	}
	pos, ok := m.doc.Position(*m.nav.Snapshot().Cursor)
	if !ok || pos.Hunk == 1 {
		t.Fatal("selection crossed into the next hunk")
	}
	if sel := m.nav.Snapshot().Selection; sel == nil {
		t.Fatal("selection unexpectedly canceled")
	} else if _, err := m.doc.Range(*sel); err != nil {
		t.Fatalf("selection became invalid: %v", err)
	}
}

func TestResizeAcrossLayoutThresholdPreservesCursorScreenRow(t *testing.T) {
	m := New(Initial{Patch: longPatch(), SideBySide: true, Width: 120, Height: 12}, Dependencies{})
	m.nav.Jump(findCell(m, "line 20 "+strings.Repeat("x", 80)))
	m.nav.Align(navigation.Middle)
	before := m.nav.Snapshot()
	beforeRow := before.Cursor.Row - before.Viewport.Top
	m.Resize(80, 12)
	m.Resize(120, 12)
	after := m.nav.Snapshot()
	if after.Cursor == nil || after.Cursor.Row-after.Viewport.Top != beforeRow {
		t.Fatalf("screen row=%d want=%d", after.Cursor.Row-after.Viewport.Top, beforeRow)
	}
}

func TestProgressAppearsOnlyAfterVerticalScroll(t *testing.T) {
	m := New(Initial{Patch: longPatch(), Width: 80, Height: 9}, Dependencies{})
	if got := m.viewLabel(); got != "local changes" {
		t.Fatalf("initial label=%q", got)
	}
	sendKey(m, "l")
	if got := m.viewLabel(); got != "local changes" {
		t.Fatalf("horizontal label=%q", got)
	}
	for m.nav.Snapshot().Viewport.Top == 0 {
		sendKey(m, "j")
	}
	if got := m.viewLabel(); !strings.HasPrefix(got, "local changes (") {
		t.Fatalf("scrolled label=%q", got)
	}
	m.nav.Last()
	if got := m.viewLabel(); got != "local changes (100%)" {
		t.Fatalf("final label=%q", got)
	}
}

func TestTabWithoutDefaultBranchDoesNothing(t *testing.T) {
	called := false
	m := New(Initial{Patch: testPatch(), Width: 120, Height: 12}, Dependencies{Load: func(string) (corepatch.Patch, error) { called = true; return testPatch(), nil }})
	cmd := sendKey(m, "tab")
	if cmd != nil || called || m.showDefault {
		t.Fatalf("cmd=%v load=%v branch=%v", cmd != nil, called, m.showDefault)
	}
}

func TestRefreshFallbackFromEmptyDiffAndPreservesCurrentLine(t *testing.T) {
	current := testPatch()
	m := New(Initial{Patch: corepatch.Patch{}, Width: 80, Height: 12}, Dependencies{Load: func(string) (corepatch.Patch, error) { return current, nil }})
	cmd := sendKey(m, "R")
	if cmd == nil {
		t.Fatal("refresh command missing")
	}
	m.Update(cmd())
	if m.nav.Snapshot().Cursor == nil || activeText(m) != "package main" {
		t.Fatal("empty diff refresh did not select first line")
	}
	m.nav.Jump(findCell(m, "new()"))
	current = testPatch()
	current.Fingerprint = "changed"
	current.Files[0].Hunks[0].Lines[2].Text = "different()"
	cmd = sendKey(m, "R")
	m.Update(cmd())
	if m.nav.Snapshot().Cursor == nil {
		t.Fatal("refresh fallback lost cursor")
	}
}

func TestCommentRequestMapsSelectedLinesToAnchor(t *testing.T) {
	m := modelForTest(80, Dependencies{})
	sendKey(m, "j")
	sendKey(m, "v")
	sendKey(m, "j")
	cmd := sendKey(m, "c")
	if cmd == nil {
		t.Fatal("comment request was not emitted")
	}
	result, ok := cmd().(CommentRequested)
	if !ok {
		t.Fatalf("result=%T", cmd())
	}
	if result.Anchor.FilePath != "main.go" || result.Anchor.OldStart != 2 || result.Anchor.NewStart != 2 || fmt.Sprint(result.Anchor.QuotedLines) != "[-old() +new()]" {
		t.Fatalf("anchor=%#v", result.Anchor)
	}
}

func TestOpenCurrentLineRequiresEditorAndUsesWorkingTreeLocation(t *testing.T) {
	m := modelForTest(80, Dependencies{})
	sendKey(m, "e")
	if m.err == nil || m.err.Error() != "$EDITOR is not set" {
		t.Fatalf("error=%v", m.err)
	}
	t.Setenv("EDITOR", "printf")
	m.err = nil
	m.nav.Jump(findCell(m, "new()"))
	if cmd := sendKey(m, "e"); cmd == nil {
		t.Fatal("source editor process was not requested")
	}
}

func TestFileJumpSequencesAndArrowNavigation(t *testing.T) {
	p := testPatch()
	p.Files = append(p.Files, corepatch.File{DisplayPath: "other.go", OldPath: "other.go", NewPath: "other.go", Hunks: []corepatch.Hunk{{Header: "@@ -1 +1 @@", Lines: []corepatch.Line{{Kind: corepatch.Context, Text: "second", OldNumber: 1, NewNumber: 1}}}}})
	m := New(Initial{Patch: p, Width: 80, Height: 12}, Dependencies{})
	first := m.doc.Row(m.nav.Snapshot().Cursor.Row).File
	sendKey(m, "]")
	sendKey(m, "f")
	if got := m.doc.Row(m.nav.Snapshot().Cursor.Row).File; got == first {
		t.Fatal("]f did not move to the next file")
	}
	sendKey(m, "[")
	sendKey(m, "f")
	if got := m.doc.Row(m.nav.Snapshot().Cursor.Row).File; got != first {
		t.Fatal("[f did not return to the previous file")
	}
}

func TestFailureIsShownOnPatchScreen(t *testing.T) {
	m := modelForTest(80, Dependencies{})
	m.Update(Failure{Err: fmt.Errorf("comment editor failed")})
	if !strings.Contains(ansi.Strip(m.Render()), "comment editor failed") {
		t.Fatal("asynchronous failure was not rendered")
	}
}
