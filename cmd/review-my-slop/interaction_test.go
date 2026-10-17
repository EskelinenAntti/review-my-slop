package main

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestAppVisualSelectionCreatesMappedAnchorAndSubmits(t *testing.T) {
	t.Setenv("EDITOR", "true")
	var saved []commentsComment
	m := newTestBasicAppModel()
	m.save = func(
		stored commentsComment,
		_ patchPatch,
	) (commentsComment, error) {
		saved = append(saved, stored)
		stored.ID = "new"
		return stored, nil
	}
	for _, key := range []string{"j", "v", "j", "c"} {
		m = pressTestKey(t, m, key)
	}
	m = updateTestAppModel(t, m, appCommentEditorFinishedMsg{
		body: "fix both lines",
	})
	requireEqual(t, len(saved), 1, "saved comments = %d", len(saved))

	anchor := saved[0].Anchor
	requireEqual(t, anchor.FilePath, "main.go", "anchor = %#v", anchor)
	requireEqual(t, anchor.OldStart, 2, "anchor = %#v", anchor)
	requireEqual(t, anchor.NewStart, 2, "anchor = %#v", anchor)
	requireEqual(t,
		slices.Equal(anchor.QuotedLines, []string{"-old()", "+new()"}), true,
		"anchor = %#v", anchor,
	)
}

func TestAppCommentSaveFailureClearsPendingEdit(t *testing.T) {
	t.Setenv("EDITOR", "true")
	m := newTestBasicAppModel()
	m.save = func(commentsComment, patchPatch) (commentsComment, error) {
		return commentsComment{}, errors.New("storage unavailable")
	}
	m = updateTestAppModel(t, m, testKeyMessage("c"))
	m = updateTestAppModel(t, m, appCommentEditorFinishedMsg{body: "keep this"})
	requireEqual(t, m.edit.body, "", "body=%q err=%v", m.edit.body, m.err)
	requireEqual(t, m.err != nil, true, "body=%q err=%v", m.edit.body, m.err)
	requireEqual(t, m.err.Error(), "storage unavailable",
		"body=%q err=%v", m.edit.body, m.err,
	)
}

func TestAppCommentRequiresEditor(t *testing.T) {
	t.Setenv("EDITOR", "")
	m := pressTestKey(t, newTestBasicAppModel(), "c")
	requireEqual(t, m.err != nil, true, "error = %v", m.err)
	requireEqual(t, m.err.Error(), "$EDITOR is not set", "error = %v", m.err)
}

func TestAppEmptyNewCommentIsDiscarded(t *testing.T) {
	t.Setenv("EDITOR", "true")
	called := false
	m := newTestBasicAppModel()
	m.save = func(
		stored commentsComment,
		_ patchPatch,
	) (commentsComment, error) {
		called = true
		return stored, nil
	}
	m = updateTestAppModel(t, m, testKeyMessage("c"))
	m = updateTestAppModel(t, m, appCommentEditorFinishedMsg{body: " \n"})
	requireEqual(t, !called, true, "called=%v comments=%d", called,
		len(m.comments.items),
	)
	requireEqual(t, len(m.comments.items), 0, "called=%v comments=%d",
		called, len(m.comments.items),
	)
}

func TestAppOpenCurrentLineUsesEditorWithWorkingTreeLocation(t *testing.T) {
	t.Setenv("EDITOR", "printf")
	m := newTestBasicAppModel()
	m.currentPatch.Root = "/tmp/repo with spaces"
	moveTestDiffToLine(t, &m, "new()")
	path, line, err := editedSourceLocation(m)
	requireEqual(t, err, nil, "location=%q:%d err=%v", path, line, err)
	requireEqual(t, path, "/tmp/repo with spaces/main.go",
		"location=%q:%d err=%v", path, line, err)
	requireEqual(t, line, 2, "location=%q:%d err=%v", path, line, err)
	cmd, err := m.startSourceEditor(m.ctx, path, line, func(err error) tea.Msg {
		return appSourceEditorFinishedMsg{err: err}
	})
	requireEqual(t, err, nil, "command=%v err=%v", cmd, err)
	requireEqual(t, cmd != nil, true, "command=%v err=%v", cmd, err)
}

func TestAppOpenCurrentLineRequiresEditor(t *testing.T) {
	t.Setenv("EDITOR", "")
	m := newTestBasicAppModel()
	m = pressTestKey(t, m, "e")
	requireEqual(t, m.err != nil, true, "error = %v", m.err)
	requireEqual(t, m.err.Error(), "$EDITOR is not set", "error = %v", m.err)
}

func TestAppCommentsCanBeViewedEditedAndDeleted(t *testing.T) {
	t.Setenv("EDITOR", "true")
	items := []commentsComment{
		{ID: "one", Body: "old body"},
		{ID: "two", Body: "second"},
	}
	var persisted, deleted commentsComment
	m := newTestBasicAppModel()
	m.comments = appCommentState{items: items}
	m.commentView = newTestCommentScreen(items)
	m.save = func(
		stored commentsComment,
		_ patchPatch,
	) (commentsComment, error) {
		persisted = stored
		return stored, nil
	}
	m.delete = func(stored commentsComment, _ patchPatch) error {
		deleted = stored
		return nil
	}
	m = updateTestAppModel(t, m, testKeyMessage("C"))
	requireEqual(t, m.mode, appModeComments, "%s", "comments did not open")
	requireEqual(
		t,
		strings.Contains(renderAppWorkflowMode(&m), "old body"),
		true,
		"%s",
		"comments did not open",
	)

	m = updateTestAppModel(t, m, tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	m = updateTestAppModel(t, m, appCommentEditorFinishedMsg{
		body: "edited body",
	})
	requireEqual(t, persisted.ID, "one", "persisted = %#v", persisted)
	requireEqual(t, persisted.Body, "edited body", "persisted = %#v", persisted)

	m = updateTestAppModel(t, m, testKeyMessage("D"))
	requireEqual(t, deleted.ID, "one", "deleted=%#v comments=%#v",
		deleted, m.comments.items,
	)
	requireEqual(t, len(m.comments.items), 1,
		"deleted=%#v comments=%#v", deleted, m.comments.items,
	)

	m = updateTestAppModel(t, m, testKeyMessage("q"))
	requireEqual(t, m.mode, appModeBrowse, "mode=%v quitting=%v", m.mode,
		m.quitting,
	)
	requireEqual(t, !m.quitting, true, "mode=%v quitting=%v", m.mode,
		m.quitting,
	)
}

func TestAppOpeningCommentsReloadsPendingComments(t *testing.T) {
	items := []commentsComment{{ID: "read", Body: "already read"}}
	m := newTestBasicAppModel()
	m.comments = appCommentState{items: items}
	m.commentView = newTestCommentScreen(items)
	m.load = func() ([]commentsComment, error) { return nil, nil }

	next, cmd := m.Update(testKeyMessage("C"))
	m = appModelFromTestResult(t, next)
	requireEqual(t, cmd != nil, true, "%s",
		"opening comments did not request a refresh",
	)

	m = runTestAppCommand(t, m, cmd)
	requireEqual(t, m.mode, appModeComments, "mode=%v comments=%#v",
		m.mode, m.comments.items,
	)
	requireEqual(t, len(m.comments.items), 0, "mode=%v comments=%#v",
		m.mode, m.comments.items,
	)
}

func TestAppCommentReloadFailurePreservesCurrentComments(t *testing.T) {
	items := []commentsComment{{ID: "keep", Body: "keep"}}
	m := newTestBasicAppModel()
	m.comments = appCommentState{items: items}
	m.commentView = newTestCommentScreen(items)
	m.load = func() ([]commentsComment, error) {
		return nil, errors.New("storage unavailable")
	}

	next, cmd := m.Update(testKeyMessage("C"))
	m = func() appModel {
		t.Helper()
		result, ok := next.(appModel)
		requireEqual(t, ok, true, "model=%T", next)
		return result
	}()
	m = runTestAppCommand(t, m, cmd)
	requireEqual(t, len(m.comments.items), 1, "comments=%#v error=%v",
		m.comments.items, m.err,
	)
	requireEqual(t, m.err != nil, true, "comments=%#v error=%v",
		m.comments.items, m.err,
	)
	requireEqual(t, m.err.Error(),
		"refresh comments: storage unavailable", "comments=%#v error=%v",
		m.comments.items, m.err,
	)
}

func TestAppEmptyEditedCommentIsDeleted(t *testing.T) {
	t.Setenv("EDITOR", "true")
	items := []commentsComment{{ID: "one", Body: "old"}}
	m := newTestBasicAppModel()
	m.comments = appCommentState{items: items}
	m.commentView = newTestCommentScreen(items)
	deleted := false
	m.delete = func(commentsComment, patchPatch) error {
		deleted = true
		return nil
	}
	m = updateTestAppModel(t, m, testKeyMessage("C"))
	m = updateTestAppModel(t, m, tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	m = updateTestAppModel(t, m, appCommentEditorFinishedMsg{body: "\n"})
	requireEqual(t, deleted, true, "deleted=%v comments=%d", deleted,
		len(m.comments.items),
	)
	requireEqual(t, len(m.comments.items), 0, "deleted=%v comments=%d",
		deleted, len(m.comments.items),
	)
}

func TestAppCommentDeleteFailureKeepsCommentAndShowsError(t *testing.T) {
	items := []commentsComment{{ID: "one", Body: "keep"}}
	m := newTestBasicAppModel()
	m.comments = appCommentState{items: items}
	m.commentView = newTestCommentScreen(items)
	m.delete = func(commentsComment, patchPatch) error {
		return errors.New("delete failed")
	}
	m = updateTestAppModel(t, m, testKeyMessage("C"))
	m = updateTestAppModel(t, m, testKeyMessage("D"))
	requireEqual(
		t,
		len(m.comments.items),
		1,
		"comments=%#v render=%q",
		m.comments.items,
		renderAppWorkflowMode(&m),
	)
	requireEqual(
		t,
		strings.Contains(
			ansi.Strip(renderAppWorkflowMode(&m)),
			"delete failed",
		),
		true,
		"comments=%#v render=%q",
		m.comments.items,
		renderAppWorkflowMode(&m),
	)
}

func TestAppSelectionCannotCrossHunk(t *testing.T) {
	m := newTestBasicAppModel()
	m = updateTestAppModel(t, m, testKeyMessage("v"))
	for range 10 {
		m = updateTestAppModel(t, m, testKeyMessage("j"))
	}
	_, line, _ := m.diffView.Current()
	requireEqual(t, line.Text != "more()", true, "%s",
		"diffSelection crossed hunk",
	)
}

func TestAppVimSequencesAndLayoutToggle(t *testing.T) {
	var saved []bool
	m := newTestBasicAppModel()
	m.saveLayout = func(enabled bool) error {
		saved = append(saved, enabled)
		return nil
	}
	m = updateTestAppModel(t, m, tea.WindowSizeMsg{Width: 120, Height: 20})
	m = updateTestAppModel(t, m, testKeyMessage("G"))
	requireEqual(
		t,
		currentTestDiffLineText(m),
		"more()",
		"G line=%q",
		currentTestDiffLineText(m),
	)

	m = pressTestAppWithG(&m, t)
	m = pressTestAppWithG(&m, t)
	requireEqual(
		t,
		currentTestDiffLineText(m),
		"package main",
		"gg line=%q",
		currentTestDiffLineText(m),
	)

	m = updateTestAppWithKey(&m, t)
	requireEqual(t, m.diffOptions.SideBySide, true, "%s",
		"split view not enabled",
	)
	requireEqual(
		t,
		strings.Contains(renderAppWorkflowMode(&m), "│"),
		true,
		"%s",
		"split view not enabled",
	)

	m = updateTestAppWithKey(&m, t)
	requireEqual(t, !m.diffOptions.SideBySide, true, "saved=%v", saved)
	requireEqual(t, slices.Equal(saved, []bool{true, false}), true,
		"saved=%v", saved,
	)
}

func TestAppSavedSideBySideCanBeDisabledInNarrowTerminal(t *testing.T) {
	m := newTestBasicAppModel()
	var saved []bool
	m.saveLayout = func(enabled bool) error {
		saved = append(saved, enabled)
		return nil
	}
	m.diffOptions.SideBySide = true
	m.diffView.Configure(m.diffOptions)
	m = updateTestAppModel(t, m, tea.WindowSizeMsg{Width: 80, Height: 20})
	m = updateTestAppWithKey(&m, t)
	requireEqual(t, !m.diffOptions.SideBySide, true,
		"sideBySide=%v saved=%v", m.diffOptions.SideBySide, saved,
	)
	requireEqual(t, slices.Equal(saved, []bool{false}), true,
		"sideBySide=%v saved=%v", m.diffOptions.SideBySide, saved,
	)
}

func TestAppResizeAcrossSideBySideThresholdPreservesFocusedLine(t *testing.T) {
	m := newTestBasicAppModel()
	m.saveLayout = nil
	m.diffOptions.SideBySide = true
	m.diffView.Configure(m.diffOptions)
	moveTestDiffToLine(t, &m, "keep()")
	_, before, _ := m.diffView.Current()
	m = updateTestAppModel(t, m, tea.WindowSizeMsg{Width: 80, Height: 20})
	m = updateTestAppModel(t, m, tea.WindowSizeMsg{Width: 120, Height: 20})
	_, got, ok := m.diffView.Current()
	requireEqual(t, ok, true, "focused line=%#v, want %#v", got, before)
	requireEqual(t, got, before, "focused line=%#v, want %#v", got, before)
}

func TestAppZSequencesPositionCurrentLineInViewport(t *testing.T) {
	for _, test := range []struct {
		key       string
		alignment diffscreenAlignment
	}{
		{
			"z", diffscreenCenter,
		}, {"t", diffscreenTop}, {"b", diffscreenBottom},
	} {
		m := newTestAppModel(makeTestPatchWithManyLines())
		m = updateTestAppModel(t, m, tea.WindowSizeMsg{Width: 100, Height: 9})
		want := newTestDiffscreenView(
			makeTestPatchWithManyLines(),
			diffscreenOptions{Dark: true},
		)
		want.Resize(100, 9)
		for range 10 {
			m.diffView.Move(diffscreenNextLine)
			want.Move(diffscreenNextLine)
		}
		want.Align(test.alignment)
		m = updateTestAppModel(t, m, testKeyMessage("z"))
		m = updateTestAppModel(t, m, testKeyMessage(test.key))
		requireEqual(t, m.diffView.Render(nil), want.Render(nil),
			"z%s did not align view", test.key,
		)
	}
}

func TestAppPendingKeyIsConsumedByNextKey(t *testing.T) {
	m := newTestBasicAppModel()
	m = updateTestAppModel(t, m, testKeyMessage("G"))
	last := currentTestDiffLineText(m)
	m = pressTestAppWithG(&m, t)
	m = updateTestAppModel(t, m, testKeyMessage("h"))
	m = pressTestAppWithG(&m, t)
	requireEqual(
		t,
		currentTestDiffLineText(m),
		last,
		"%s",
		"pending prefix moved focus",
	)

	m = pressTestAppWithG(&m, t)
	requireEqual(
		t,
		currentTestDiffLineText(m),
		"package main",
		"gg line=%q",
		currentTestDiffLineText(m),
	)
}

func TestAppStatusShowsBasicBindingsAndHelpShowsCompleteKeyMap(t *testing.T) {
	m := newTestBasicAppModel()
	status := ansi.Strip(strings.Split(renderTestAppMode(m), "\n")[m.height-2])
	requireEqual(t, strings.HasPrefix(status, "j/k/h/l move"), true,
		"status=%q", status,
	)
	requireEqual(t, strings.HasSuffix(status, "local changes"), true,
		"status=%q", status,
	)

	help := ansi.Strip(renderTestHelpBindings(
		expectedTestHelpBindings(), m.width, m.height,
	))
	for _, binding := range []string{
		"Ctrl-w h/l/w",
		"zz/zt/zb",
		"n/N",
		"]f/[f",
		"Tab",
		"t",
	} {
		requireEqual(t, strings.Contains(help, binding), true,
			"help missing %q", binding,
		)
	}
}

func TestAppStatusShowsProgressOnlyAfterViewportMoves(t *testing.T) {
	m := newTestAppModel(makeTestPatchWithManyLines())
	m = updateTestAppModel(t, m, tea.WindowSizeMsg{Width: 80, Height: 9})
	requireEqual(
		t,
		!strings.Contains(renderAppWorkflowMode(&m), "%"),
		true,
		"%s",
		"initial view shows progress",
	)

	m = updateTestAppModel(t, m, testKeyMessage("l"))
	requireEqual(
		t,
		!strings.Contains(renderAppWorkflowMode(&m), "%"),
		true,
		"%s",
		"horizontal scroll shows progress",
	)

	for range 10 {
		m = updateTestAppModel(t, m, testKeyMessage("j"))
	}
	requireEqual(
		t,
		strings.Contains(renderAppWorkflowMode(&m), "%)"),
		true,
		"%s",
		"scrolled view hides progress",
	)

	m = updateTestAppModel(t, m, testKeyMessage("G"))
	requireEqual(
		t,
		strings.Contains(renderAppWorkflowMode(&m), "local changes (100%)"),
		true,
		"%s",
		"final view hides progress",
	)
}

func TestAppStatusHidesProgressWhenDiffFitsViewport(t *testing.T) {
	m := newTestBasicAppModel()
	m = updateTestAppModel(t, m, tea.WindowSizeMsg{Width: 100, Height: 100})
	requireEqual(
		t,
		!strings.Contains(renderAppWorkflowMode(&m), "%)"),
		true,
		"%s",
		"fitting diff shows progress",
	)
}

func TestAppSideBySidePaneSwitchingUsesCtrlWSequences(t *testing.T) {
	m := newTestBasicAppModel()
	m = updateTestAppModel(t, m, tea.WindowSizeMsg{Width: 120, Height: 30})
	m.diffOptions.SideBySide = true
	m.diffView.Configure(m.diffOptions)
	moveTestDiffToLine(t, &m, "new()")
	m = updateTestAppModel(t, m, tea.KeyPressMsg(tea.Key{
		Code: 'w',
		Mod:  tea.ModCtrl,
	}))
	m = updateTestAppModel(t, m, testKeyMessage("h"))
	requireEqual(
		t,
		currentTestDiffLineText(m),
		"old()",
		"old pane line=%q",
		currentTestDiffLineText(m),
	)

	m = updateTestAppModel(t, m, tea.KeyPressMsg(tea.Key{
		Code: 'w',
		Mod:  tea.ModCtrl,
	}))
	m = updateTestAppModel(t, m, tea.KeyPressMsg(tea.Key{
		Code: 'w',
		Mod:  tea.ModCtrl,
	}))
	requireEqual(
		t,
		currentTestDiffLineText(m),
		"new()",
		"new pane line=%q",
		currentTestDiffLineText(m),
	)
}

func TestAppHorizontalScrollKeysMoveByStepAndReset(t *testing.T) {
	m := newTestAppModel(makeTestPatchWithManyLines())
	m = updateTestAppModel(t, m, tea.WindowSizeMsg{Width: 37, Height: 30})
	initial := m.diffView.Render(nil)
	want := newTestDiffscreenView(
		makeTestPatchWithManyLines(),
		diffscreenOptions{Dark: true},
	)
	want.Resize(37, 30)
	want.ScrollHorizontal(2 * appHorizontalScrollStep)
	m = updateTestAppModel(t, m, testKeyMessage("l"))
	m = updateTestAppModel(t, m, tea.KeyPressMsg(tea.Key{Code: tea.KeyRight}))
	requireEqual(t, m.diffView.Render(nil), want.Render(nil), "%s",
		"right keys did not scroll by two steps",
	)

	m = updateTestAppModel(t, m, testKeyMessage("h"))
	m = updateTestAppModel(t, m, tea.KeyPressMsg(tea.Key{Code: tea.KeyLeft}))
	requireEqual(t, m.diffView.Render(nil), initial, "%s",
		"left keys did not restore initial offset",
	)

	m = updateTestAppModel(t, m, testKeyMessage("$"))
	requireEqual(t, m.diffView.Render(nil) != initial, true, "%s",
		"$ did not scroll",
	)

	m = updateTestAppModel(t, m, testKeyMessage("0"))
	requireEqual(t, m.diffView.Render(nil), initial, "%s",
		"0 did not restore initial offset",
	)
}

func TestAppFocusAndManualRefreshLoadCurrentView(t *testing.T) {
	m := newTestBasicAppModel()
	m.currentPatch.Branch = "main"
	m.kind = patchBranch
	var requested []patchKind
	m.refresh = func(kind patchKind) (patchPatch, error) {
		requested = append(requested, kind)
		patch := newTestBasicPatch()
		patch.Kind, patch.Branch = kind, "main"
		patch.Files[0].Metadata = []string{
			fmt.Sprintf("refresh-%d", len(requested)),
		}
		return patch, nil
	}
	next, cmd := m.Update(tea.FocusMsg{})
	m = appModelFromTestResult(t, next)
	requireEqual(t, cmd != nil, true, "%s", "focus did not refresh")

	m = runTestAppCommand(t, m, cmd)
	next, cmd = m.Update(testKeyMessage("R"))
	m = appModelFromTestResult(t, next)
	requireEqual(t, cmd != nil, true, "%s", "R did not refresh")

	m = runTestAppCommand(t, m, cmd)
	requireEqual(t,
		slices.Equal(requested, []patchKind{patchBranch, patchBranch}), true,
		"requested=%v metadata=%q", requested, m.currentPatch.Files[0].Metadata,
	)
	requireEqual(t,
		slices.Equal(
			m.currentPatch.Files[0].Metadata,
			[]string{"refresh-2"},
		), true,
		"requested=%v metadata=%q", requested, m.currentPatch.Files[0].Metadata,
	)
}

func TestAppSourceEditorCompletionRefreshesDiff(t *testing.T) {
	m := newTestBasicAppModel()
	refreshed := newTestBasicPatch()
	refreshed.Files[0].Metadata = []string{"after-editor"}
	m.refresh = func(patchKind) (patchPatch, error) { return refreshed, nil }

	next, cmd := m.Update(appSourceEditorFinishedMsg{})
	m = appModelFromTestResult(t, next)
	requireEqual(t, cmd != nil, true, "%s", "editor completion did not refresh")

	m = runTestAppCommand(t, m, cmd)
	requireEqual(t,
		slices.Equal(
			m.currentPatch.Files[0].Metadata,
			[]string{"after-editor"},
		),
		true, "metadata=%q", m.currentPatch.Files[0].Metadata,
	)
}

func TestAppHeaderShowsAddedAndRemovedLineCounts(t *testing.T) {
	m := newTestBasicAppModel()
	header := strings.SplitN(
		ansi.Strip(renderAppWorkflowMode(&m)),
		"\n",
		2,
	)[0]
	requireEqual(t, header, "review-my-slop  +2-1", "header = %q", header)
}

func TestAppSearchMovesIncrementallyRepeatsAndRestoresOrigin(t *testing.T) {
	m := newTestBasicAppModel()
	origin := testRenderedDiffBody(m)
	m = updateTestAppModel(t, m, testKeyMessage("/"))
	m = updateTestAppModel(t, m, testKeyMessage("keep"))
	first := testRenderedDiffBody(m)
	requireEqual(
		t,
		m.mode,
		appModeSearch,
		"mode=%v line=%q",
		m.mode,
		currentTestDiffLineText(m),
	)
	requireEqual(
		t,
		currentTestDiffLineText(m),
		"keep()",
		"mode=%v line=%q",
		m.mode,
		currentTestDiffLineText(m),
	)

	m = updateTestAppModel(t, m, tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	m = updateTestAppModel(t, m, testKeyMessage("n"))
	requireEqual(t, testRenderedDiffBody(m) != first, true,
		"next=%#v", testRenderedDiffBody(m),
	)
	requireEqual(
		t,
		currentTestDiffLineText(m),
		"keep()",
		"next=%#v",
		testRenderedDiffBody(m),
	)

	m = updateTestAppModel(t, m, testKeyMessage("N"))
	requireEqual(t, testRenderedDiffBody(m), first,
		"previous=%#v", testRenderedDiffBody(m),
	)

	m = updateTestAppModel(t, m, testKeyMessage("/"))
	m = updateTestAppModel(t, m, testKeyMessage("missing"))
	requireEqual(
		t,
		strings.Contains(renderAppWorkflowMode(&m), "no matches"),
		true,
		"%s",
		"missing search did not miss",
	)

	m = updateTestAppModel(t, m, tea.KeyPressMsg(tea.Key{Code: tea.KeyEsc}))
	requireEqual(t, testRenderedDiffBody(m), first,
		"cancel=%#v origin=%#v", testRenderedDiffBody(m), origin,
	)
}

func TestAppSearchMatchesFileNamesAndBackspaceRestoresOrigin(t *testing.T) {
	m := newTestBasicAppModel()
	origin := testRenderedDiffBody(m)
	m = updateTestAppModel(t, m, testKeyMessage("/"))
	m = updateTestAppModel(t, m, testKeyMessage("main.go"))
	file, _, _ := m.diffView.Current()
	requireEqual(t, file.DisplayPath, "main.go", "file=%q", file.DisplayPath)

	for range len("main.go") {
		m = updateTestAppModel(t, m, tea.KeyPressMsg(tea.Key{
			Code: tea.KeyBackspace,
		}))
	}
	requireEqual(t, testRenderedDiffBody(m), origin,
		"diffCursor=%#v", testRenderedDiffBody(m),
	)
}

func TestAppSideBySideSearchActivatesPaneAndCancelRestoresIt(t *testing.T) {
	m := newTestBasicAppModel()
	m = updateTestAppModel(t, m, tea.WindowSizeMsg{Width: 120, Height: 30})
	m.diffOptions.SideBySide = true
	m.diffView.Configure(m.diffOptions)
	origin := testRenderedDiffBody(m)
	m = updateTestAppModel(t, m, testKeyMessage("/"))
	m = updateTestAppModel(t, m, testKeyMessage("old()"))
	requireEqual(
		t,
		currentTestDiffLineText(m),
		"old()",
		"diffCursor=%#v line=%q",
		testRenderedDiffBody(m),
		currentTestDiffLineText(m),
	)

	m = updateTestAppModel(t, m, tea.KeyPressMsg(tea.Key{Code: tea.KeyEsc}))
	requireEqual(t, testRenderedDiffBody(m), origin,
		"cancel=%#v want=%#v", testRenderedDiffBody(m), origin,
	)
}

func TestAppTabTogglesDefaultBranchAndIgnoresStaleRefresh(t *testing.T) {
	m := newTestBasicAppModel()
	m.currentPatch.Branch = "main"
	m.refresh = func(kind patchKind) (patchPatch, error) {
		patch := newTestBasicPatch()
		patch.Kind, patch.Branch = kind, "main"
		return patch, nil
	}
	m = updateTestAppModel(t, m, testKeyMessage("tab"))
	requireEqual(t, m.kind, patchBranch, "kind=%v", m.kind)

	stale := newTestBasicPatch()
	stale.Files[0].Metadata = []string{"stale"}
	m = updateTestAppModel(t, m, appRefreshDiffMsg{patch: stale})
	requireEqual(t,
		!slices.Equal(
			m.currentPatch.Files[0].Metadata,
			[]string{"stale"},
		), true,
		"%s", "stale refresh applied",
	)

	m = updateTestAppModel(t, m, testKeyMessage("tab"))
	requireEqual(
		t,
		m.kind,
		patchUnstaged,
		"kind=%v after toggling back to local",
		m.kind,
	)
}

func TestAppTabDoesNothingWithoutDefaultBranch(t *testing.T) {
	m := newTestBasicAppModel()
	refreshed := false
	m.refresh = func(patchKind) (patchPatch, error) {
		refreshed = true
		return newTestBasicPatch(), nil
	}
	next, cmd := m.Update(testKeyMessage("tab"))
	m = appModelFromTestResult(t, next)
	requireEqual(t, cmd == nil, true,
		"tab changed model without default branch: cmd=%v refreshed=%v kind=%v",
		cmd != nil, refreshed, m.kind,
	)
	requireEqual(t, !refreshed, true,
		"tab changed model without default branch: cmd=%v refreshed=%v kind=%v",
		cmd != nil, refreshed, m.kind,
	)
	requireEqual(
		t,
		m.kind,
		patchUnstaged,
		"tab changed model without default branch: cmd=%v refreshed=%v kind=%v",
		cmd != nil,
		refreshed,
		m.kind,
	)
}

func TestAppDiffRefreshFallbackAndEmptyDiff(t *testing.T) {
	m := newTestAppModel(patchPatch{})
	m = updateTestAppModel(t, m, appRefreshDiffMsg{
		patch: newTestBasicPatch(),
	})
	requireEqual(
		t,
		currentTestDiffLineText(m),
		"package main",
		"initial focus=%q",
		currentTestDiffLineText(m),
	)

	moveTestDiffToLine(t, &m, "new()")
	changed := newTestBasicPatch()
	changed.Files[0].Hunks[0].Lines[2].Text = "different()"
	m = updateTestAppModel(t, m, appRefreshDiffMsg{patch: changed})
	if _, _, ok := m.diffView.Current(); !ok {
		t.Fatal("refresh lost focus")
	}
}

func TestAppCommentAfterRefreshUsesCurrentPatch(t *testing.T) {
	t.Setenv("EDITOR", "true")
	var saved patchPatch
	m := newTestBasicAppModel()
	m.save = func(
		stored commentsComment,
		patch patchPatch,
	) (commentsComment, error) {
		saved = patch
		return stored, nil
	}
	refreshed := newTestBasicPatch()
	refreshed.Files[0].Metadata = []string{"refreshed"}
	m = updateTestAppModel(t, m, appRefreshDiffMsg{patch: refreshed})
	m = updateTestAppModel(t, m, testKeyMessage("c"))
	m = updateTestAppModel(t, m, appCommentEditorFinishedMsg{body: "comment"})
	requireEqual(t,
		slices.Equal(saved.Files[0].Metadata, []string{"refreshed"}), true,
		"metadata=%q", saved.Files[0].Metadata,
	)
}

func TestAppViewPreservesTerminalColors(t *testing.T) {
	result := newTestBasicAppModel().View()
	requireEqual(t, result.BackgroundColor, nil, "view=%#v", result)
	requireEqual(t, result.ForegroundColor, nil, "view=%#v", result)
	requireEqual(t, result.AltScreen, true, "view=%#v", result)
	requireEqual(t, result.ReportFocus, true, "view=%#v", result)
	requireEqual(t, result.MouseMode, tea.MouseModeCellMotion, "%s",
		"view does not capture terminal wheel events",
	)
}

func TestAppMouseWheelScrollsContentWithoutMovingFocus(t *testing.T) {
	m := newTestAppModel(makeTestPatchWithManyLines())
	m = updateTestAppModel(t, m, tea.WindowSizeMsg{Width: 37, Height: 9})
	m = updateTestAppModel(t, m, testKeyMessage("v"))
	m = updateTestAppModel(t, m, testKeyMessage("j"))
	_, focus, _ := m.diffView.Current()
	_, selected, _ := m.diffView.Selected()
	initial := renderAppWorkflowMode(&m)
	for _, button := range []tea.MouseButton{
		tea.MouseWheelDown,
		tea.MouseWheelRight,
	} {
		m = updateTestAppModel(t, m, tea.MouseWheelMsg{Button: button})
	}
	requireEqual(
		t,
		renderAppWorkflowMode(&m) != initial,
		true,
		"%s",
		"wheel did not scroll content",
	)
	requireEqual(
		t,
		strings.Contains(renderAppWorkflowMode(&m), "%)"),
		true,
		"%s",
		"wheel did not scroll content",
	)

	_, got, _ := m.diffView.Current()
	_, gotSelected, _ := m.diffView.Selected()
	requireEqual(t, got, focus, "%s", "wheel changed focus or selection")
	requireEqual(t, slices.Equal(selected, gotSelected), true, "%s",
		"wheel changed focus or selection",
	)

	for _, button := range []tea.MouseButton{
		tea.MouseWheelUp,
		tea.MouseWheelLeft,
	} {
		m = updateTestAppModel(t, m, tea.MouseWheelMsg{Button: button})
	}
	requireEqual(
		t,
		renderAppWorkflowMode(&m),
		initial,
		"%s",
		"reverse wheel did not restore content",
	)

	for _, inlineLocalMode1 := range []appMode{appModeComments, appModeHelp} {
		m.mode = inlineLocalMode1
		before := m.diffView.Render(nil)
		m = updateTestAppModel(t, m, tea.MouseWheelMsg{
			Button: tea.MouseWheelDown,
		})
		requireEqual(t, m.diffView.Render(nil), before, "%s",
			"wheel scrolled diff behind another screen",
		)
	}
}
