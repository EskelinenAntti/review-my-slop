package app

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/eskelinenantti/review-my-slop/internal/comments"
	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/ui/diffscreen"
)

func TestVisualSelectionCreatesMappedAnchorAndSubmits(t *testing.T) {
	t.Setenv("EDITOR", "true")
	var saved []comments.Comment
	m := testModel(coveragePatch(), nil, func(stored comments.Comment, _ patch.Patch) (comments.Comment, error) {
		saved = append(saved, stored)
		stored.ID = "new"
		return stored, nil
	})
	m = updateModel(t, m, textKey("j"))
	m = updateModel(t, m, textKey("v"))
	m = updateModel(t, m, textKey("j"))
	m = updateModel(t, m, textKey("c"))
	_ = updateModel(t, m, commentEditorFinishedMsg{body: "fix both lines"})
	if len(saved) != 1 {
		t.Fatalf("saved comments = %d", len(saved))
	}
	anchor := saved[0].Anchor
	if anchor.FilePath != "main.go" || anchor.OldStart != 2 || anchor.NewStart != 2 || !slices.Equal(anchor.QuotedLines, []string{"-old()", "+new()"}) {
		t.Fatalf("anchor = %#v", anchor)
	}
}

func TestCommentSaveFailureClearsPendingEdit(t *testing.T) {
	t.Setenv("EDITOR", "true")
	m := testModel(coveragePatch(), nil, func(comments.Comment, patch.Patch) (comments.Comment, error) {
		return comments.Comment{}, fmt.Errorf("storage unavailable")
	})
	m = updateModel(t, m, textKey("c"))
	m = updateModel(t, m, commentEditorFinishedMsg{body: "keep this"})
	if m.edit.body != "" || m.err == nil || m.err.Error() != "storage unavailable" {
		t.Fatalf("body=%q err=%v", m.edit.body, m.err)
	}
}

func TestCommentRequiresEditor(t *testing.T) {
	t.Setenv("EDITOR", "")
	m := updateModel(t, testModel(coveragePatch(), nil, nil), textKey("c"))
	if m.err == nil || m.err.Error() != "$EDITOR is not set" {
		t.Fatalf("error = %v", m.err)
	}
}

func TestEmptyNewCommentIsDiscarded(t *testing.T) {
	t.Setenv("EDITOR", "true")
	called := false
	m := testModel(coveragePatch(), nil, func(stored comments.Comment, p patch.Patch) (comments.Comment, error) {
		called = true
		return stored, nil
	})
	m = updateModel(t, m, textKey("c"))
	m = updateModel(t, m, commentEditorFinishedMsg{body: " \n"})
	if called || len(m.comments.items) != 0 {
		t.Fatalf("called=%v comments=%d", called, len(m.comments.items))
	}
}

func TestOpenCurrentLineUsesEditorWithWorkingTreeLocation(t *testing.T) {
	t.Setenv("EDITOR", "printf")
	m := testModel(coveragePatch(), nil, nil)
	m.currentPatch.Root = "/tmp/repo with spaces"
	focusLine(t, &m, "new()")
	cmd, err := m.openCurrentLine()
	if err != nil || cmd == nil {
		t.Fatalf("command=%v err=%v", cmd, err)
	}

	path, line, err := m.sourceLocation()
	if err != nil || path != "/tmp/repo with spaces/main.go" || line != 2 {
		t.Fatalf("location=%q:%d err=%v", path, line, err)
	}
}

func TestOpenCurrentLineRequiresEditor(t *testing.T) {
	t.Setenv("EDITOR", "")
	m := updateModel(t, testModel(coveragePatch(), nil, nil), textKey("e"))
	if m.err == nil || m.err.Error() != "$EDITOR is not set" {
		t.Fatalf("error = %v", m.err)
	}
}

func TestCommentsCanBeViewedEditedAndDeleted(t *testing.T) {
	t.Setenv("EDITOR", "true")
	items := []comments.Comment{{ID: "one", Body: "old body"}, {ID: "two", Body: "second"}}
	var persisted, deleted comments.Comment
	m := testModel(coveragePatch(), items, func(stored comments.Comment, _ patch.Patch) (comments.Comment, error) {
		persisted = stored
		return stored, nil
	})
	m.delete = func(stored comments.Comment, _ patch.Patch) error { deleted = stored; return nil }
	m = updateModel(t, m, textKey("C"))
	if m.mode != modeComments || !strings.Contains(m.render(), "old body") {
		t.Fatal("comments did not open")
	}
	m = updateModel(t, m, specialKey(tea.KeyEnter))
	m = updateModel(t, m, commentEditorFinishedMsg{body: "edited body"})
	if persisted.ID != "one" || persisted.Body != "edited body" {
		t.Fatalf("persisted = %#v", persisted)
	}
	m = updateModel(t, m, textKey("D"))
	if deleted.ID != "one" || len(m.comments.items) != 1 {
		t.Fatalf("deleted=%#v comments=%#v", deleted, m.comments.items)
	}
	m = updateModel(t, m, textKey("q"))
	if m.mode != modeBrowse || m.quitting {
		t.Fatalf("mode=%v quitting=%v", m.mode, m.quitting)
	}
}

func TestOpeningCommentsReloadsPendingComments(t *testing.T) {
	m := testModel(coveragePatch(), []comments.Comment{{ID: "read", Body: "already read"}}, nil)
	m.load = func() ([]comments.Comment, error) { return nil, nil }

	next, cmd := m.Update(textKey("C"))
	m = next.(model)
	if cmd == nil {
		t.Fatal("opening comments did not request a refresh")
	}
	m = updateModel(t, m, cmd())
	if m.mode != modeComments || len(m.comments.items) != 0 {
		t.Fatalf("mode=%v comments=%#v", m.mode, m.comments.items)
	}
}

func TestCommentReloadFailurePreservesCurrentComments(t *testing.T) {
	m := testModel(coveragePatch(), []comments.Comment{{ID: "keep", Body: "keep"}}, nil)
	m.load = func() ([]comments.Comment, error) { return nil, fmt.Errorf("storage unavailable") }

	next, cmd := m.Update(textKey("C"))
	m = next.(model)
	m = updateModel(t, m, cmd())
	if len(m.comments.items) != 1 || m.err == nil || m.err.Error() != "refresh comments: storage unavailable" {
		t.Fatalf("comments=%#v error=%v", m.comments.items, m.err)
	}
}

func TestEmptyEditedCommentIsDeleted(t *testing.T) {
	t.Setenv("EDITOR", "true")
	m := testModel(coveragePatch(), []comments.Comment{{ID: "one", Body: "old"}}, nil)
	deleted := false
	m.delete = func(comments.Comment, patch.Patch) error { deleted = true; return nil }
	m = updateModel(t, m, textKey("C"))
	m = updateModel(t, m, specialKey(tea.KeyEnter))
	m = updateModel(t, m, commentEditorFinishedMsg{body: "\n"})
	if !deleted || len(m.comments.items) != 0 {
		t.Fatalf("deleted=%v comments=%d", deleted, len(m.comments.items))
	}
}

func TestCommentDeleteFailureKeepsCommentAndShowsError(t *testing.T) {
	m := testModel(coveragePatch(), []comments.Comment{{ID: "one", Body: "keep"}}, nil)
	m.delete = func(comments.Comment, patch.Patch) error { return fmt.Errorf("delete failed") }
	m = updateModel(t, m, textKey("C"))
	m = updateModel(t, m, textKey("D"))
	if len(m.comments.items) != 1 || !strings.Contains(ansi.Strip(m.render()), "delete failed") {
		t.Fatalf("comments=%#v render=%q", m.comments.items, m.render())
	}
}

func TestSelectionCannotCrossHunk(t *testing.T) {
	m := testModel(coveragePatch(), nil, nil)
	m = updateModel(t, m, textKey("v"))
	for range 10 {
		m = updateModel(t, m, textKey("j"))
	}
	_, line, _ := m.diffView.Current()
	if line.Text == "more()" {
		t.Fatal("diffSelection crossed hunk")
	}
}

func TestVimSequencesAndLayoutToggle(t *testing.T) {
	m := testModel(coveragePatch(), nil, nil)
	var saved []bool
	m.saveLayout = func(enabled bool) error { saved = append(saved, enabled); return nil }
	m.setSideBySide(false)
	m = updateModel(t, m, tea.WindowSizeMsg{Width: 120, Height: 20})
	m = updateModel(t, m, textKey("G"))
	if lineText(m) != "more()" {
		t.Fatalf("G line=%q", lineText(m))
	}
	m = updateModel(t, m, textKey("g"))
	m = updateModel(t, m, textKey("g"))
	if lineText(m) != "package main" {
		t.Fatalf("gg line=%q", lineText(m))
	}
	m = updateModel(t, m, textKey("t"))
	if !m.diffOptions.SideBySide || !strings.Contains(m.render(), "│") {
		t.Fatal("split view not enabled")
	}
	m = updateModel(t, m, textKey("t"))
	if m.diffOptions.SideBySide || !slices.Equal(saved, []bool{true, false}) {
		t.Fatalf("saved=%v", saved)
	}
}

func TestSavedSideBySideCanBeDisabledInNarrowTerminal(t *testing.T) {
	m := testModel(coveragePatch(), nil, nil)
	var saved []bool
	m.saveLayout = func(enabled bool) error { saved = append(saved, enabled); return nil }
	m.setSideBySide(true)
	m = updateModel(t, m, tea.WindowSizeMsg{Width: 80, Height: 20})
	m = updateModel(t, m, textKey("t"))
	if m.diffOptions.SideBySide || !slices.Equal(saved, []bool{false}) {
		t.Fatalf("sideBySide=%v saved=%v", m.diffOptions.SideBySide, saved)
	}
}

func TestResizeAcrossSideBySideThresholdPreservesFocusedLine(t *testing.T) {
	m := testModel(coveragePatch(), nil, nil)
	m.saveLayout = nil
	m.setSideBySide(true)
	focusLine(t, &m, "keep()")
	_, before, _ := m.diffView.Current()
	m = updateModel(t, m, tea.WindowSizeMsg{Width: 80, Height: 20})
	m = updateModel(t, m, tea.WindowSizeMsg{Width: 120, Height: 20})
	_, got, ok := m.diffView.Current()
	if !ok || got != before {
		t.Fatalf("focused line=%#v, want %#v", got, before)
	}
}

func TestZSequencesPositionCurrentLineInViewport(t *testing.T) {
	for _, test := range []struct {
		key       string
		alignment diffscreen.Alignment
	}{
		{"z", diffscreen.Center}, {"t", diffscreen.Top}, {"b", diffscreen.Bottom},
	} {
		m := testModel(longModelPatch(), nil, nil)
		m = updateModel(t, m, tea.WindowSizeMsg{Width: 100, Height: 9})
		want := diffscreen.New(longModelPatch(), diffscreen.Options{Dark: true})
		want.Resize(100, 9)
		for range 10 {
			m.diffView.Move(diffscreen.NextLine)
			want.Move(diffscreen.NextLine)
		}
		want.Align(test.alignment)
		m = updateModel(t, m, textKey("z"))
		m = updateModel(t, m, textKey(test.key))
		if m.diffView.Render(nil) != want.Render(nil) {
			t.Fatalf("z%s did not align view", test.key)
		}
	}
}

func TestPendingKeyIsConsumedByNextKey(t *testing.T) {
	m := testModel(coveragePatch(), nil, nil)
	m = updateModel(t, m, textKey("G"))
	last := lineText(m)
	m = updateModel(t, m, textKey("g"))
	m = updateModel(t, m, textKey("h"))
	m = updateModel(t, m, textKey("g"))
	if lineText(m) != last {
		t.Fatal("pending prefix moved focus")
	}
	m = updateModel(t, m, textKey("g"))
	if lineText(m) != "package main" {
		t.Fatalf("gg line=%q", lineText(m))
	}
}

func TestStatusShowsBasicBindingsAndHelpShowsCompleteKeyMap(t *testing.T) {
	m := testModel(coveragePatch(), nil, nil)
	status := ansi.Strip(strings.Split(m.render(), "\n")[m.height-2])
	if !strings.HasPrefix(status, "j/k/h/l move") || !strings.HasSuffix(status, "local changes") {
		t.Fatalf("status=%q", status)
	}
	help := ansi.Strip(m.renderHelp())
	for _, binding := range []string{"Ctrl-w h/l/w", "zz/zt/zb", "n/N", "]f/[f", "Tab", "t"} {
		if !strings.Contains(help, binding) {
			t.Fatalf("help missing %q", binding)
		}
	}
}

func TestStatusShowsProgressOnlyAfterViewportMoves(t *testing.T) {
	m := testModel(longModelPatch(), nil, nil)
	m = updateModel(t, m, tea.WindowSizeMsg{Width: 80, Height: 9})
	if strings.Contains(m.render(), "%") {
		t.Fatal("initial view shows progress")
	}
	m = updateModel(t, m, textKey("l"))
	if strings.Contains(m.render(), "%") {
		t.Fatal("horizontal scroll shows progress")
	}
	for range 10 {
		m = updateModel(t, m, textKey("j"))
	}
	if !strings.Contains(m.render(), "%)") {
		t.Fatal("scrolled view hides progress")
	}
	m = updateModel(t, m, textKey("G"))
	if !strings.Contains(m.render(), "local changes (100%)") {
		t.Fatal("final view hides progress")
	}
}

func TestStatusHidesProgressWhenDiffFitsViewport(t *testing.T) {
	m := testModel(coveragePatch(), nil, nil)
	m = updateModel(t, m, tea.WindowSizeMsg{Width: 100, Height: 100})
	if strings.Contains(m.render(), "%)") {
		t.Fatal("fitting diff shows progress")
	}
}

func TestSideBySidePaneSwitchingUsesCtrlWSequences(t *testing.T) {
	m := testModel(coveragePatch(), nil, nil)
	m = updateModel(t, m, tea.WindowSizeMsg{Width: 120, Height: 30})
	m.setSideBySide(true)
	focusLine(t, &m, "new()")
	m = updateModel(t, m, controlKey('w'))
	m = updateModel(t, m, textKey("h"))
	if lineText(m) != "old()" {
		t.Fatalf("old pane line=%q", lineText(m))
	}
	m = updateModel(t, m, controlKey('w'))
	m = updateModel(t, m, controlKey('w'))
	if lineText(m) != "new()" {
		t.Fatalf("new pane line=%q", lineText(m))
	}
}

func TestHorizontalScrollKeysMoveByStepAndReset(t *testing.T) {
	m := testModel(longModelPatch(), nil, nil)
	m = updateModel(t, m, tea.WindowSizeMsg{Width: 37, Height: 30})
	initial := m.diffView.Render(nil)
	want := diffscreen.New(longModelPatch(), diffscreen.Options{Dark: true})
	want.Resize(37, 30)
	want.ScrollHorizontal(2 * horizontalScrollStep)
	m = updateModel(t, m, textKey("l"))
	m = updateModel(t, m, tea.KeyPressMsg(tea.Key{Code: tea.KeyRight}))
	if m.diffView.Render(nil) != want.Render(nil) {
		t.Fatal("right keys did not scroll by two steps")
	}
	m = updateModel(t, m, textKey("h"))
	m = updateModel(t, m, tea.KeyPressMsg(tea.Key{Code: tea.KeyLeft}))
	if m.diffView.Render(nil) != initial {
		t.Fatal("left keys did not restore initial offset")
	}
	m = updateModel(t, m, textKey("$"))
	if m.diffView.Render(nil) == initial {
		t.Fatal("$ did not scroll")
	}
	m = updateModel(t, m, textKey("0"))
	if m.diffView.Render(nil) != initial {
		t.Fatal("0 did not restore initial offset")
	}
}

func TestFocusAndManualRefreshLoadCurrentView(t *testing.T) {
	m := testModel(coveragePatch(), nil, nil)
	m.currentPatch.Branch = "main"
	m.kind = patch.Branch
	var requested []patch.Kind
	m.refresh = func(kind patch.Kind) (patch.Patch, error) {
		requested = append(requested, kind)
		p := coveragePatch()
		p.Kind, p.Branch = kind, "main"
		p.Files[0].Metadata = []string{fmt.Sprintf("refresh-%d", len(requested))}
		return p, nil
	}
	next, cmd := m.Update(tea.FocusMsg{})
	m = next.(model)
	if cmd == nil {
		t.Fatal("focus did not refresh")
	}
	m = updateModel(t, m, cmd())
	next, cmd = m.Update(textKey("R"))
	m = next.(model)
	if cmd == nil {
		t.Fatal("R did not refresh")
	}
	m = updateModel(t, m, cmd())
	if !slices.Equal(requested, []patch.Kind{patch.Branch, patch.Branch}) || !slices.Equal(m.currentPatch.Files[0].Metadata, []string{"refresh-2"}) {
		t.Fatalf("requested=%v metadata=%q", requested, m.currentPatch.Files[0].Metadata)
	}
}

func TestSourceEditorCompletionRefreshesDiff(t *testing.T) {
	m := testModel(coveragePatch(), nil, nil)
	refreshed := coveragePatch()
	refreshed.Files[0].Metadata = []string{"after-editor"}
	m.refresh = func(patch.Kind) (patch.Patch, error) { return refreshed, nil }

	next, cmd := m.Update(sourceEditorFinishedMsg{})
	m = next.(model)
	if cmd == nil {
		t.Fatal("editor completion did not refresh")
	}
	m = updateModel(t, m, cmd())
	if !slices.Equal(m.currentPatch.Files[0].Metadata, []string{"after-editor"}) {
		t.Fatalf("metadata=%q", m.currentPatch.Files[0].Metadata)
	}
}

func TestHeaderShowsAddedAndRemovedLineCounts(t *testing.T) {
	header := strings.SplitN(ansi.Strip(testModel(coveragePatch(), nil, nil).render()), "\n", 2)[0]
	if header != "review-my-slop  +2-1" {
		t.Fatalf("header = %q", header)
	}
}

func TestSearchMovesIncrementallyRepeatsAndRestoresOrigin(t *testing.T) {
	m := testModel(coveragePatch(), nil, nil)
	origin := screenBody(m.diffView.Render(nil))
	m = updateModel(t, m, textKey("/"))
	m = updateModel(t, m, textKey("keep"))
	first := screenBody(m.diffView.Render(nil))
	if m.mode != modeSearch || lineText(m) != "keep()" {
		t.Fatalf("mode=%v line=%q", m.mode, lineText(m))
	}
	m = updateModel(t, m, specialKey(tea.KeyEnter))
	m = updateModel(t, m, textKey("n"))
	if screenBody(m.diffView.Render(nil)) == first || lineText(m) != "keep()" {
		t.Fatalf("next=%#v", screenBody(m.diffView.Render(nil)))
	}
	m = updateModel(t, m, textKey("N"))
	if screenBody(m.diffView.Render(nil)) != first {
		t.Fatalf("previous=%#v", screenBody(m.diffView.Render(nil)))
	}
	m = updateModel(t, m, textKey("/"))
	m = updateModel(t, m, textKey("missing"))
	if !strings.Contains(m.render(), "no matches") {
		t.Fatal("missing search did not miss")
	}
	m = updateModel(t, m, specialKey(tea.KeyEsc))
	if screenBody(m.diffView.Render(nil)) != first {
		t.Fatalf("cancel=%#v origin=%#v", screenBody(m.diffView.Render(nil)), origin)
	}
}

func TestSearchMatchesFileNamesAndBackspaceRestoresOrigin(t *testing.T) {
	m := testModel(coveragePatch(), nil, nil)
	origin := screenBody(m.diffView.Render(nil))
	m = updateModel(t, m, textKey("/"))
	m = updateModel(t, m, textKey("main.go"))
	file, _, _ := m.diffView.Current()
	if file.DisplayPath != "main.go" {
		t.Fatalf("file=%q", file.DisplayPath)
	}
	for range len("main.go") {
		m = updateModel(t, m, specialKey(tea.KeyBackspace))
	}
	if screenBody(m.diffView.Render(nil)) != origin {
		t.Fatalf("diffCursor=%#v", screenBody(m.diffView.Render(nil)))
	}
}

func TestSideBySideSearchActivatesPaneAndCancelRestoresIt(t *testing.T) {
	m := testModel(coveragePatch(), nil, nil)
	m = updateModel(t, m, tea.WindowSizeMsg{Width: 120, Height: 30})
	m.setSideBySide(true)
	origin := screenBody(m.diffView.Render(nil))
	m = updateModel(t, m, textKey("/"))
	m = updateModel(t, m, textKey("old()"))
	if lineText(m) != "old()" {
		t.Fatalf("diffCursor=%#v line=%q", screenBody(m.diffView.Render(nil)), lineText(m))
	}
	m = updateModel(t, m, specialKey(tea.KeyEsc))
	if screenBody(m.diffView.Render(nil)) != origin {
		t.Fatalf("cancel=%#v want=%#v", screenBody(m.diffView.Render(nil)), origin)
	}
}

func TestTabTogglesDefaultBranchAndIgnoresStaleRefresh(t *testing.T) {
	m := testModel(coveragePatch(), nil, nil)
	m.currentPatch.Branch = "main"
	m.refresh = func(kind patch.Kind) (patch.Patch, error) {
		p := coveragePatch()
		p.Kind, p.Branch = kind, "main"
		return p, nil
	}
	next, _ := m.Update(textKey("tab"))
	m = next.(model)
	if m.kind != patch.Branch {
		t.Fatalf("kind=%v", m.kind)
	}
	stale := coveragePatch()
	stale.Files[0].Metadata = []string{"stale"}
	m = updateModel(t, m, refreshDiffMsg{patch: stale})
	if slices.Equal(m.currentPatch.Files[0].Metadata, []string{"stale"}) {
		t.Fatal("stale refresh applied")
	}
	m = updateModel(t, m, textKey("tab"))
	if m.kind != patch.Unstaged {
		t.Fatalf("kind=%v after toggling back to local", m.kind)
	}
}

func TestTabDoesNothingWithoutDefaultBranch(t *testing.T) {
	m := testModel(coveragePatch(), nil, nil)
	refreshed := false
	m.refresh = func(patch.Kind) (patch.Patch, error) {
		refreshed = true
		return coveragePatch(), nil
	}
	next, cmd := m.Update(textKey("tab"))
	m = next.(model)
	if cmd != nil || refreshed || m.kind != patch.Unstaged {
		t.Fatalf("tab changed model without default branch: cmd=%v refreshed=%v kind=%v", cmd != nil, refreshed, m.kind)
	}
}

func TestDiffRefreshFallbackAndEmptyDiff(t *testing.T) {
	m := testModel(patch.Patch{}, nil, nil)
	m = updateModel(t, m, refreshDiffMsg{patch: coveragePatch()})
	if lineText(m) != "package main" {
		t.Fatalf("initial focus=%q", lineText(m))
	}
	focusLine(t, &m, "new()")
	changed := coveragePatch()
	changed.Files[0].Hunks[0].Lines[2].Text = "different()"
	m = updateModel(t, m, refreshDiffMsg{patch: changed})
	if _, _, ok := m.diffView.Current(); !ok {
		t.Fatal("refresh lost focus")
	}
}

func TestCommentAfterRefreshUsesCurrentPatch(t *testing.T) {
	t.Setenv("EDITOR", "true")
	var saved patch.Patch
	m := testModel(coveragePatch(), nil, func(stored comments.Comment, p patch.Patch) (comments.Comment, error) {
		saved = p
		return stored, nil
	})
	refreshed := coveragePatch()
	refreshed.Files[0].Metadata = []string{"refreshed"}
	m = updateModel(t, m, refreshDiffMsg{patch: refreshed})
	m = updateModel(t, m, textKey("c"))
	_ = updateModel(t, m, commentEditorFinishedMsg{body: "comment"})
	if !slices.Equal(saved.Files[0].Metadata, []string{"refreshed"}) {
		t.Fatalf("metadata=%q", saved.Files[0].Metadata)
	}
}

func TestViewPreservesTerminalColors(t *testing.T) {
	result := testModel(coveragePatch(), nil, nil).View()
	if result.BackgroundColor != nil || result.ForegroundColor != nil || !result.AltScreen || !result.ReportFocus {
		t.Fatalf("view=%#v", result)
	}
	if result.MouseMode != tea.MouseModeCellMotion {
		t.Fatal("view does not capture terminal wheel events")
	}
}

func TestMouseWheelScrollsContentWithoutMovingFocus(t *testing.T) {
	m := testModel(longModelPatch(), nil, nil)
	m = updateModel(t, m, tea.WindowSizeMsg{Width: 37, Height: 9})
	m = updateModel(t, m, textKey("v"))
	m = updateModel(t, m, textKey("j"))
	_, focus, _ := m.diffView.Current()
	_, selected, _ := m.diffView.Selected()
	initial := m.render()
	for _, button := range []tea.MouseButton{tea.MouseWheelDown, tea.MouseWheelRight} {
		m = updateModel(t, m, tea.MouseWheelMsg{Button: button})
	}
	if m.render() == initial || !strings.Contains(m.render(), "%)") {
		t.Fatal("wheel did not scroll content")
	}
	_, got, _ := m.diffView.Current()
	_, gotSelected, _ := m.diffView.Selected()
	if got != focus || !slices.Equal(selected, gotSelected) {
		t.Fatal("wheel changed focus or selection")
	}
	for _, button := range []tea.MouseButton{tea.MouseWheelUp, tea.MouseWheelLeft} {
		m = updateModel(t, m, tea.MouseWheelMsg{Button: button})
	}
	if m.render() != initial {
		t.Fatal("reverse wheel did not restore content")
	}
	for _, mode := range []mode{modeComments, modeHelp} {
		m.mode = mode
		before := m.diffView.Render(nil)
		m = updateModel(t, m, tea.MouseWheelMsg{Button: tea.MouseWheelDown})
		if m.diffView.Render(nil) != before {
			t.Fatal("wheel scrolled diff behind another screen")
		}
	}
}

func updateModel(t *testing.T, m model, msg tea.Msg) model {
	t.Helper()
	next, _ := m.Update(msg)
	result, ok := next.(model)
	if !ok {
		t.Fatalf("model=%T", next)
	}
	return result
}
func textKey(text string) tea.KeyPressMsg {
	runes := []rune(text)
	return tea.KeyPressMsg(tea.Key{Text: text, Code: runes[0]})
}
func specialKey(code rune) tea.KeyPressMsg { return tea.KeyPressMsg(tea.Key{Code: code}) }
func controlKey(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Code: code, Mod: tea.ModCtrl})
}

func focusLine(t *testing.T, m *model, text string) {
	t.Helper()
	m.diffView.Move(diffscreen.FirstLine)
	count := 0
	for _, file := range m.currentPatch.Files {
		for _, hunk := range file.Hunks {
			count += len(hunk.Lines)
		}
	}
	for range count {
		if lineText(*m) == text {
			return
		}
		m.diffView.Move(diffscreen.NextLine)
	}
	t.Fatalf("line %q not found", text)
}

func lineText(m model) string { _, line, _ := m.diffView.Current(); return line.Text }

func screenBody(rendered string) string {
	lines := strings.Split(rendered, "\n")
	return strings.Join(lines[1:len(lines)-2], "\n")
}

func coveragePatch() patch.Patch {
	return patch.Patch{Root: "/repo", Files: []patch.File{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", OldSource: "package main\nold()\nkeep()\n", NewSource: "package main\nnew()\nkeep()\nmore()\n", Hunks: []patch.Hunk{
		{Header: "@@ -1,3 +1,3 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "package main", OldNumber: 1, NewNumber: 1}, {Kind: patch.Deletion, Text: "old()", OldNumber: 2}, {Kind: patch.Addition, Text: "new()", NewNumber: 2}, {Kind: patch.Context, Text: "keep()", OldNumber: 3, NewNumber: 3}}},
		{Header: "@@ -3,1 +3,2 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "keep()", OldNumber: 3, NewNumber: 3}, {Kind: patch.Addition, Text: "more()", NewNumber: 4}}},
	}}}}
}

func longModelPatch() patch.Patch {
	lines := make([]patch.Line, 30)
	for index := range lines {
		lines[index] = patch.Line{Kind: patch.Context, Text: fmt.Sprintf("line %d %s", index, strings.Repeat("x", 80)), OldNumber: patch.LineNumber(index + 1), NewNumber: patch.LineNumber(index + 1)}
	}
	return patch.Patch{Files: []patch.File{{DisplayPath: "long.go", Hunks: []patch.Hunk{{Header: "@@", Lines: lines}}}}}
}
