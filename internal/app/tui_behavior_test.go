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
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
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
	testutil.Requiref(t, len(saved) == 1, "saved comments = %d", len(saved))
	anchor := saved[0].Anchor
	testutil.Requiref(t, anchor.FilePath == "main.go" && anchor.OldStart == 2 && anchor.NewStart == 2 && slices.Equal(anchor.QuotedLines, []string{"-old()", "+new()"}), "anchor = %#v", anchor)
}

func TestCommentSaveFailureClearsPendingEdit(t *testing.T) {
	t.Setenv("EDITOR", "true")
	m := testModel(coveragePatch(), nil, func(comments.Comment, patch.Patch) (comments.Comment, error) {
		return comments.Comment{}, fmt.Errorf("storage unavailable")
	})
	m = updateModel(t, m, textKey("c"))
	m = updateModel(t, m, commentEditorFinishedMsg{body: "keep this"})
	testutil.Requiref(t, m.edit.body == "" && m.err != nil && m.err.Error() == "storage unavailable", "body=%q err=%v", m.edit.body, m.err)
}

func TestCommentRequiresEditor(t *testing.T) {
	t.Setenv("EDITOR", "")
	m := updateModel(t, testModel(coveragePatch(), nil, nil), textKey("c"))
	testutil.Requiref(t, m.err != nil && m.err.Error() == "$EDITOR is not set", "error = %v", m.err)
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
	testutil.Requiref(t, !called && len(m.comments.items) == 0, "called=%v comments=%d", called, len(m.comments.items))
}

func TestOpenCurrentLineUsesEditorWithWorkingTreeLocation(t *testing.T) {
	t.Setenv("EDITOR", "printf")
	m := testModel(coveragePatch(), nil, nil)
	m.currentPatch.Root = "/tmp/repo with spaces"
	focusLine(t, &m, "new()")
	cmd, err := m.openCurrentLine()
	testutil.Requiref(t, err == nil && cmd != nil, "command=%v err=%v", cmd, err)

	path, line, err := m.sourceLocation()
	testutil.Requiref(t, err == nil && path == "/tmp/repo with spaces/main.go" && line == 2, "location=%q:%d err=%v", path, line, err)
}

func TestOpenCurrentLineRequiresEditor(t *testing.T) {
	t.Setenv("EDITOR", "")
	m := updateModel(t, testModel(coveragePatch(), nil, nil), textKey("e"))
	testutil.Requiref(t, m.err != nil && m.err.Error() == "$EDITOR is not set", "error = %v", m.err)
}

func TestCommentsCanBeViewedEditedAndDeleted(t *testing.T) {
	t.Setenv("EDITOR", "true")
	items := []comments.Comment{{ID: "one", Body: "old body"}, {ID: "two", Body: "second"}}
	var persisted, deleted comments.Comment
	m := testModel(coveragePatch(), items, func(stored comments.Comment, _ patch.Patch) (comments.Comment, error) {
		persisted = stored
		return stored, nil
	})
	m.setDelete(func(stored comments.Comment, _ patch.Patch) error { deleted = stored; return nil })
	m = updateModel(t, m, textKey("C"))
	testutil.Require(t, m.mode == modeComments && strings.Contains(m.render(), "old body"), "comments did not open")
	m = updateModel(t, m, specialKey(tea.KeyEnter))
	m = updateModel(t, m, commentEditorFinishedMsg{body: "edited body"})
	testutil.Requiref(t, persisted.ID == "one" && persisted.Body == "edited body", "persisted = %#v", persisted)
	m = updateModel(t, m, textKey("D"))
	testutil.Requiref(t, deleted.ID == "one" && len(m.comments.items) == 1, "deleted=%#v comments=%#v", deleted, m.comments.items)
	m = updateModel(t, m, textKey("q"))
	testutil.Requiref(t, m.mode == modeBrowse && !m.quitting, "mode=%v quitting=%v", m.mode, m.quitting)
}

func TestOpeningCommentsReloadsPendingComments(t *testing.T) {
	m := testModel(coveragePatch(), []comments.Comment{{ID: "read", Body: "already read"}}, nil)
	m.setLoadComments(func() ([]comments.Comment, error) { return nil, nil })

	next, cmd := m.Update(textKey("C"))
	m = next.(model)
	testutil.Require(t, cmd != nil, "opening comments did not request a refresh")
	m = updateModel(t, m, cmd())
	testutil.Requiref(t, m.mode == modeComments && len(m.comments.items) == 0, "mode=%v comments=%#v", m.mode, m.comments.items)
}

func TestCommentReloadFailurePreservesCurrentComments(t *testing.T) {
	m := testModel(coveragePatch(), []comments.Comment{{ID: "keep", Body: "keep"}}, nil)
	m.setLoadComments(func() ([]comments.Comment, error) { return nil, fmt.Errorf("storage unavailable") })

	next, cmd := m.Update(textKey("C"))
	m = next.(model)
	m = updateModel(t, m, cmd())
	testutil.Requiref(t, len(m.comments.items) == 1 && m.err != nil && m.err.Error() == "refresh comments: storage unavailable", "comments=%#v error=%v", m.comments.items, m.err)
}

func TestEmptyEditedCommentIsDeleted(t *testing.T) {
	t.Setenv("EDITOR", "true")
	m := testModel(coveragePatch(), []comments.Comment{{ID: "one", Body: "old"}}, nil)
	deleted := false
	m.setDelete(func(comments.Comment, patch.Patch) error { deleted = true; return nil })
	m = updateModel(t, m, textKey("C"))
	m = updateModel(t, m, specialKey(tea.KeyEnter))
	m = updateModel(t, m, commentEditorFinishedMsg{body: "\n"})
	testutil.Requiref(t, deleted && len(m.comments.items) == 0, "deleted=%v comments=%d", deleted, len(m.comments.items))
}

func TestCommentDeleteFailureKeepsCommentAndShowsError(t *testing.T) {
	m := testModel(coveragePatch(), []comments.Comment{{ID: "one", Body: "keep"}}, nil)
	m.setDelete(func(comments.Comment, patch.Patch) error { return fmt.Errorf("delete failed") })
	m = updateModel(t, m, textKey("C"))
	m = updateModel(t, m, textKey("D"))
	testutil.Requiref(t, len(m.comments.items) == 1 && strings.Contains(ansi.Strip(m.render()), "delete failed"), "comments=%#v render=%q", m.comments.items, m.render())
}

func TestSelectionCannotCrossHunk(t *testing.T) {
	m := testModel(coveragePatch(), nil, nil)
	m = updateModel(t, m, textKey("v"))
	for range 10 {
		m = updateModel(t, m, textKey("j"))
	}
	_, line, _ := m.diffView.Current()
	testutil.Require(t, line.Text != "more()", "diffSelection crossed hunk")
}

func TestVimSequencesAndLayoutToggle(t *testing.T) {
	m := testModel(coveragePatch(), nil, nil)
	var saved []bool
	m.configureSideBySide(false, func(enabled bool) error { saved = append(saved, enabled); return nil })
	m = updateModel(t, m, tea.WindowSizeMsg{Width: 120, Height: 20})
	m = updateModel(t, m, textKey("G"))
	testutil.Requiref(t, lineText(m) == "more()", "G line=%q", lineText(m))
	m = updateModel(t, m, textKey("g"))
	m = updateModel(t, m, textKey("g"))
	testutil.Requiref(t, lineText(m) == "package main", "gg line=%q", lineText(m))
	m = updateModel(t, m, textKey("t"))
	testutil.Require(t, m.diffOptions.SideBySide && strings.Contains(m.render(), "│"), "split view not enabled")
	m = updateModel(t, m, textKey("t"))
	testutil.Requiref(t, !m.diffOptions.SideBySide && slices.Equal(saved, []bool{true, false}), "saved=%v", saved)
}

func TestSavedSideBySideCanBeDisabledInNarrowTerminal(t *testing.T) {
	m := testModel(coveragePatch(), nil, nil)
	var saved []bool
	m.configureSideBySide(true, func(enabled bool) error { saved = append(saved, enabled); return nil })
	m = updateModel(t, m, tea.WindowSizeMsg{Width: 80, Height: 20})
	m = updateModel(t, m, textKey("t"))
	testutil.Requiref(t, !m.diffOptions.SideBySide && slices.Equal(saved, []bool{false}), "sideBySide=%v saved=%v", m.diffOptions.SideBySide, saved)
}

func TestResizeAcrossSideBySideThresholdPreservesFocusedLine(t *testing.T) {
	m := testModel(coveragePatch(), nil, nil)
	m.configureSideBySide(true, nil)
	focusLine(t, &m, "keep()")
	_, before, _ := m.diffView.Current()
	m = updateModel(t, m, tea.WindowSizeMsg{Width: 80, Height: 20})
	m = updateModel(t, m, tea.WindowSizeMsg{Width: 120, Height: 20})
	_, got, ok := m.diffView.Current()
	testutil.Requiref(t, ok && got == before, "focused line=%#v, want %#v", got, before)
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
		testutil.Requiref(t, m.diffView.Render(nil) == want.Render(nil), "z%s did not align view", test.key)
	}
}

func TestPendingKeyIsConsumedByNextKey(t *testing.T) {
	m := testModel(coveragePatch(), nil, nil)
	m = updateModel(t, m, textKey("G"))
	last := lineText(m)
	m = updateModel(t, m, textKey("g"))
	m = updateModel(t, m, textKey("h"))
	m = updateModel(t, m, textKey("g"))
	testutil.Require(t, lineText(m) == last, "pending prefix moved focus")
	m = updateModel(t, m, textKey("g"))
	testutil.Requiref(t, lineText(m) == "package main", "gg line=%q", lineText(m))
}

func TestStatusShowsBasicBindingsAndHelpShowsCompleteKeyMap(t *testing.T) {
	m := testModel(coveragePatch(), nil, nil)
	status := ansi.Strip(strings.Split(m.render(), "\n")[m.height-2])
	testutil.Requiref(t, strings.HasPrefix(status, "j/k/h/l move") && strings.HasSuffix(status, "local changes"), "status=%q", status)
	help := ansi.Strip(m.renderHelp())
	for _, binding := range []string{"Ctrl-w h/l/w", "zz/zt/zb", "n/N", "]f/[f", "Tab", "t"} {
		testutil.Requiref(t, strings.Contains(help, binding), "help missing %q", binding)
	}
}

func TestStatusShowsProgressOnlyAfterViewportMoves(t *testing.T) {
	m := testModel(longModelPatch(), nil, nil)
	m = updateModel(t, m, tea.WindowSizeMsg{Width: 80, Height: 9})
	testutil.Require(t, !strings.Contains(m.render(), "%"), "initial view shows progress")
	m = updateModel(t, m, textKey("l"))
	testutil.Require(t, !strings.Contains(m.render(), "%"), "horizontal scroll shows progress")
	for range 10 {
		m = updateModel(t, m, textKey("j"))
	}
	testutil.Require(t, strings.Contains(m.render(), "%)"), "scrolled view hides progress")
	m = updateModel(t, m, textKey("G"))
	testutil.Require(t, strings.Contains(m.render(), "local changes (100%)"), "final view hides progress")
}

func TestStatusHidesProgressWhenDiffFitsViewport(t *testing.T) {
	m := testModel(coveragePatch(), nil, nil)
	m = updateModel(t, m, tea.WindowSizeMsg{Width: 100, Height: 100})
	testutil.Require(t, !strings.Contains(m.render(), "%)"), "fitting diff shows progress")
}

func TestSideBySidePaneSwitchingUsesCtrlWSequences(t *testing.T) {
	m := testModel(coveragePatch(), nil, nil)
	m = updateModel(t, m, tea.WindowSizeMsg{Width: 120, Height: 30})
	m.setSideBySide(true)
	focusLine(t, &m, "new()")
	m = updateModel(t, m, controlKey('w'))
	m = updateModel(t, m, textKey("h"))
	testutil.Requiref(t, lineText(m) == "old()", "old pane line=%q", lineText(m))
	m = updateModel(t, m, controlKey('w'))
	m = updateModel(t, m, controlKey('w'))
	testutil.Requiref(t, lineText(m) == "new()", "new pane line=%q", lineText(m))
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
	testutil.Require(t, m.diffView.Render(nil) == want.Render(nil), "right keys did not scroll by two steps")
	m = updateModel(t, m, textKey("h"))
	m = updateModel(t, m, tea.KeyPressMsg(tea.Key{Code: tea.KeyLeft}))
	testutil.Require(t, m.diffView.Render(nil) == initial, "left keys did not restore initial offset")
	m = updateModel(t, m, textKey("$"))
	testutil.Require(t, m.diffView.Render(nil) != initial, "$ did not scroll")
	m = updateModel(t, m, textKey("0"))
	testutil.Require(t, m.diffView.Render(nil) == initial, "0 did not restore initial offset")
}

func TestFocusAndManualRefreshLoadCurrentView(t *testing.T) {
	m := testModel(coveragePatch(), nil, nil)
	m.currentPatch.Branch = "main"
	m.kind = patch.Branch
	var requested []patch.Kind
	m.setRefresh(func(kind patch.Kind) (patch.Patch, error) {
		requested = append(requested, kind)
		p := coveragePatch()
		p.Kind, p.Branch = kind, "main"
		p.Files[0].Metadata = []string{fmt.Sprintf("refresh-%d", len(requested))}
		return p, nil
	})
	next, cmd := m.Update(tea.FocusMsg{})
	m = next.(model)
	testutil.Require(t, cmd != nil, "focus did not refresh")
	m = updateModel(t, m, cmd())
	next, cmd = m.Update(textKey("R"))
	m = next.(model)
	testutil.Require(t, cmd != nil, "R did not refresh")
	m = updateModel(t, m, cmd())
	testutil.Requiref(t, slices.Equal(requested, []patch.Kind{patch.Branch, patch.Branch}) && slices.Equal(m.currentPatch.Files[0].Metadata, []string{"refresh-2"}), "requested=%v metadata=%q", requested, m.currentPatch.Files[0].Metadata)
}

func TestSourceEditorCompletionRefreshesDiff(t *testing.T) {
	m := testModel(coveragePatch(), nil, nil)
	refreshed := coveragePatch()
	refreshed.Files[0].Metadata = []string{"after-editor"}
	m.setRefresh(func(patch.Kind) (patch.Patch, error) { return refreshed, nil })

	next, cmd := m.Update(sourceEditorFinishedMsg{})
	m = next.(model)
	testutil.Require(t, cmd != nil, "editor completion did not refresh")
	m = updateModel(t, m, cmd())
	testutil.Requiref(t, slices.Equal(m.currentPatch.Files[0].Metadata, []string{"after-editor"}), "metadata=%q", m.currentPatch.Files[0].Metadata)
}

func TestHeaderShowsAddedAndRemovedLineCounts(t *testing.T) {
	header := strings.SplitN(ansi.Strip(testModel(coveragePatch(), nil, nil).render()), "\n", 2)[0]
	testutil.Requiref(t, header == "review-my-slop  +2-1", "header = %q", header)
}

func TestSearchMovesIncrementallyRepeatsAndRestoresOrigin(t *testing.T) {
	m := testModel(coveragePatch(), nil, nil)
	origin := screenBody(m.diffView.Render(nil))
	m = updateModel(t, m, textKey("/"))
	m = updateModel(t, m, textKey("keep"))
	first := screenBody(m.diffView.Render(nil))
	testutil.Requiref(t, m.mode == modeSearch && lineText(m) == "keep()", "mode=%v line=%q", m.mode, lineText(m))
	m = updateModel(t, m, specialKey(tea.KeyEnter))
	m = updateModel(t, m, textKey("n"))
	testutil.Requiref(t, screenBody(m.diffView.Render(nil)) != first && lineText(m) == "keep()", "next=%#v", screenBody(m.diffView.Render(nil)))
	m = updateModel(t, m, textKey("N"))
	testutil.Requiref(t, screenBody(m.diffView.Render(nil)) == first, "previous=%#v", screenBody(m.diffView.Render(nil)))
	m = updateModel(t, m, textKey("/"))
	m = updateModel(t, m, textKey("missing"))
	testutil.Require(t, strings.Contains(m.render(), "no matches"), "missing search did not miss")
	m = updateModel(t, m, specialKey(tea.KeyEsc))
	testutil.Requiref(t, screenBody(m.diffView.Render(nil)) == first, "cancel=%#v origin=%#v", screenBody(m.diffView.Render(nil)), origin)
}

func TestSearchMatchesFileNamesAndBackspaceRestoresOrigin(t *testing.T) {
	m := testModel(coveragePatch(), nil, nil)
	origin := screenBody(m.diffView.Render(nil))
	m = updateModel(t, m, textKey("/"))
	m = updateModel(t, m, textKey("main.go"))
	file, _, _ := m.diffView.Current()
	testutil.Requiref(t, file.DisplayPath == "main.go", "file=%q", file.DisplayPath)
	for range len("main.go") {
		m = updateModel(t, m, specialKey(tea.KeyBackspace))
	}
	testutil.Requiref(t, screenBody(m.diffView.Render(nil)) == origin, "diffCursor=%#v", screenBody(m.diffView.Render(nil)))
}

func TestSideBySideSearchActivatesPaneAndCancelRestoresIt(t *testing.T) {
	m := testModel(coveragePatch(), nil, nil)
	m = updateModel(t, m, tea.WindowSizeMsg{Width: 120, Height: 30})
	m.setSideBySide(true)
	origin := screenBody(m.diffView.Render(nil))
	m = updateModel(t, m, textKey("/"))
	m = updateModel(t, m, textKey("old()"))
	testutil.Requiref(t, lineText(m) == "old()", "diffCursor=%#v line=%q", screenBody(m.diffView.Render(nil)), lineText(m))
	m = updateModel(t, m, specialKey(tea.KeyEsc))
	testutil.Requiref(t, screenBody(m.diffView.Render(nil)) == origin, "cancel=%#v want=%#v", screenBody(m.diffView.Render(nil)), origin)
}

func TestTabTogglesDefaultBranchAndIgnoresStaleRefresh(t *testing.T) {
	m := testModel(coveragePatch(), nil, nil)
	m.currentPatch.Branch = "main"
	m.setRefresh(func(kind patch.Kind) (patch.Patch, error) {
		p := coveragePatch()
		p.Kind, p.Branch = kind, "main"
		return p, nil
	})
	next, _ := m.Update(textKey("tab"))
	m = next.(model)
	testutil.Requiref(t, m.kind == patch.Branch, "kind=%v", m.kind)
	stale := coveragePatch()
	stale.Files[0].Metadata = []string{"stale"}
	m = updateModel(t, m, refreshDiffMsg{patch: stale})
	testutil.Require(t, !slices.Equal(m.currentPatch.Files[0].Metadata, []string{"stale"}), "stale refresh applied")
	m = updateModel(t, m, textKey("tab"))
	testutil.Requiref(t, m.kind == patch.Unstaged, "kind=%v after toggling back to local", m.kind)
}

func TestTabDoesNothingWithoutDefaultBranch(t *testing.T) {
	m := testModel(coveragePatch(), nil, nil)
	refreshed := false
	m.setRefresh(func(patch.Kind) (patch.Patch, error) {
		refreshed = true
		return coveragePatch(), nil
	})
	next, cmd := m.Update(textKey("tab"))
	m = next.(model)
	testutil.Requiref(t, cmd == nil && !refreshed && m.kind == patch.Unstaged, "tab changed model without default branch: cmd=%v refreshed=%v kind=%v", cmd != nil, refreshed, m.kind)
}

func TestDiffRefreshFallbackAndEmptyDiff(t *testing.T) {
	m := testModel(patch.Patch{}, nil, nil)
	m = updateModel(t, m, refreshDiffMsg{patch: coveragePatch()})
	testutil.Requiref(t, lineText(m) == "package main", "initial focus=%q", lineText(m))
	focusLine(t, &m, "new()")
	changed := coveragePatch()
	changed.Files[0].Hunks[0].Lines[2].Text = "different()"
	m = updateModel(t, m, refreshDiffMsg{patch: changed})
	{
		_, _, ok := m.diffView.Current()
		testutil.Require(t, ok, "refresh lost focus")
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
	testutil.Requiref(t, slices.Equal(saved.Files[0].Metadata, []string{"refreshed"}), "metadata=%q", saved.Files[0].Metadata)
}

func TestViewPreservesTerminalColors(t *testing.T) {
	result := testModel(coveragePatch(), nil, nil).View()
	testutil.Requiref(t, result.BackgroundColor == nil && result.ForegroundColor == nil && result.AltScreen && result.ReportFocus, "view=%#v", result)
	testutil.Require(t, result.MouseMode == tea.MouseModeCellMotion, "view does not capture terminal wheel events")
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
	testutil.Require(t, m.render() != initial && strings.Contains(m.render(), "%)"), "wheel did not scroll content")
	_, got, _ := m.diffView.Current()
	_, gotSelected, _ := m.diffView.Selected()
	testutil.Require(t, got == focus && slices.Equal(selected, gotSelected), "wheel changed focus or selection")
	for _, button := range []tea.MouseButton{tea.MouseWheelUp, tea.MouseWheelLeft} {
		m = updateModel(t, m, tea.MouseWheelMsg{Button: button})
	}
	testutil.Require(t, m.render() == initial, "reverse wheel did not restore content")
	assertWheelIgnoresOtherScreens(t, m)
}

func updateModel(t *testing.T, m model, msg tea.Msg) model {
	t.Helper()
	next, _ := m.Update(msg)
	result, ok := next.(model)
	testutil.Requiref(t, ok, "model=%T", next)
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
	count := patchCodeLineCount(m.currentPatch)
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

func assertWheelIgnoresOtherScreens(t *testing.T, m model) {
	t.Helper()
	for _, mode := range []mode{modeComments, modeHelp} {
		m.mode = mode
		before := m.diffView.Render(nil)
		m = updateModel(t, m, tea.MouseWheelMsg{Button: tea.MouseWheelDown})
		testutil.Require(t, m.diffView.Render(nil) == before, "wheel scrolled diff behind another screen")
	}
}

func patchCodeLineCount(p patch.Patch) int {
	count := 0
	for _, file := range p.Files {
		count += fileCodeLineCount(file)
	}
	return count
}

func fileCodeLineCount(file patch.File) int {
	count := 0
	for _, hunk := range file.Hunks {
		count += len(hunk.Lines)
	}
	return count
}
