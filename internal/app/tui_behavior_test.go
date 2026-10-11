package app

import (
	"errors"
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
	t.Setenv(editorVariable, fixtureTrue)
	var saved []comments.Comment
	m := testModel(
		coveragePatch(),
		nil,
		func(
			stored comments.Comment,
			_ patch.Patch,
		) (
			comments.Comment,
			error,
		) {
			saved = append(saved, stored)
			stored.ID = "new"
			return stored, nil
		},
	)
	m = updateModel(t, m, textKey(keyJ))
	m = updateModel(t, m, textKey(selectionKey))
	m = updateModel(t, m, textKey(keyJ))
	m = updateModel(t, m, textKey(commentKey))
	_ = updateModel(t, m, commentEditorFinishedMsg{body: "fix both lines"})
	requireEqual(t, len(saved), one, "saved comments = %d", len(saved))
	anchor := saved[zero].Anchor
	requireEqual(
		t,
		anchor.FilePath == fixtureMainGo && anchor.OldStart == two &&
			anchor.NewStart == two &&
			slices.Equal(anchor.QuotedLines, []string{"-old()", "+new()"}),
		true,
		"anchor = %#v",
		anchor,
	)
}

func TestCommentSaveFailureClearsPendingEdit(t *testing.T) {
	t.Setenv(editorVariable, fixtureTrue)
	m := testModel(
		coveragePatch(),
		nil,
		func(comments.Comment, patch.Patch) (
			comments.Comment,
			error,
		) {
			return comments.Comment{}, errors.New("storage unavailable")
		},
	)
	m = updateModel(t, m, textKey(commentKey))
	m = updateModel(t, m, commentEditorFinishedMsg{body: "keep this"})
	requireEqual(
		t,
		m.edit.body == "" && m.err != nil &&
			m.err.Error() == "storage unavailable",
		true,
		"body=%q err=%v",
		m.edit.body,
		m.err,
	)
}

func TestCommentRequiresEditor(t *testing.T) {
	t.Setenv(editorVariable, "")
	m := updateModel(
		t,
		testModel(coveragePatch(), nil, nil),
		textKey(commentKey),
	)
	requireEqual(
		t,
		m.err != nil && m.err.Error() == "$EDITOR is not set", true,
		"error = %v",
		m.err,
	)
}

func TestEmptyNewCommentIsDiscarded(t *testing.T) {
	t.Setenv(editorVariable, fixtureTrue)
	called := false
	m := testModel(
		coveragePatch(),
		nil,
		func(
			stored comments.Comment,
			_ patch.Patch,
		) (
			comments.Comment,
			error,
		) {
			called = true
			return stored, nil
		},
	)
	m = updateModel(t, m, textKey(commentKey))
	m = updateModel(t, m, commentEditorFinishedMsg{body: " \n"})
	requireEqual(
		t,
		!called && len(m.comments.items) == zero, true,
		"called=%v comments=%d",
		called,
		len(m.comments.items),
	)
}

func TestOpenCurrentLineUsesEditorWithWorkingTreeLocation(t *testing.T) {
	t.Setenv(editorVariable, "printf")
	m := testModel(coveragePatch(), nil, nil)
	m.currentPatch.Root = "/tmp/repo with spaces"
	focusLine(t, &m, fixtureNew)
	cmd, err := m.openCurrentLine()
	requireEqual(
		t,
		err == nil && cmd != nil, true,
		"command=%v err=%v",
		cmd,
		err,
	)

	path, line, err := m.sourceLocation()
	requireEqual(
		t,
		err == nil && path == "/tmp/repo with spaces/main.go" && line == two,
		true,
		"location=%q:%d err=%v",
		path,
		line,
		err,
	)
}

func TestOpenCurrentLineRequiresEditor(t *testing.T) {
	t.Setenv(editorVariable, "")
	m := updateModel(t, testModel(coveragePatch(), nil, nil), textKey("e"))
	requireEqual(
		t,
		m.err != nil && m.err.Error() == "$EDITOR is not set", true,
		"error = %v",
		m.err,
	)
}

func TestCommentsCanBeViewedEditedAndDeleted(t *testing.T) {
	t.Setenv(editorVariable, fixtureTrue)
	items := []comments.Comment{
		{ID: "one", Body: "old body"},
		{ID: "two", Body: "second"},
	}
	var persisted, deleted comments.Comment
	m := testModel(
		coveragePatch(),
		items,
		func(
			stored comments.Comment,
			_ patch.Patch,
		) (
			comments.Comment,
			error,
		) {
			persisted = stored
			return stored, nil
		},
	)
	m.setDelete(
		func(stored comments.Comment, _ patch.Patch) error {
			deleted = stored
			return nil
		},
	)
	m = updateModel(t, m, textKey(commentsKey))
	requireEqual(
		t,
		m.mode == modeComments && strings.Contains(m.render(), "old body"),
		true,
		valueFormat,
		"comments did not open",
	)
	m = updateModel(t, m, specialKey(tea.KeyEnter))
	m = updateModel(t, m, commentEditorFinishedMsg{body: "edited body"})
	requireEqual(
		t,
		persisted.ID == "one" && persisted.Body == "edited body", true,
		"persisted = %#v",
		persisted,
	)
	m = updateModel(t, m, textKey("D"))
	requireEqual(
		t,
		deleted.ID == "one" && len(m.comments.items) == one, true,
		"deleted=%#v comments=%#v",
		deleted,
		m.comments.items,
	)
	m = updateModel(t, m, textKey("q"))
	requireEqual(
		t,
		m.mode == modeBrowse && !m.quitting, true,
		"mode=%v quitting=%v",
		m.mode,
		m.quitting,
	)
}

func TestOpeningCommentsReloadsPendingComments(t *testing.T) {
	m := testModel(
		coveragePatch(),
		[]comments.Comment{{ID: "read", Body: "already read"}},
		nil,
	)
	m.setLoadComments(func() ([]comments.Comment, error) { return nil, nil })

	next, cmd := m.Update(textKey(commentsKey))
	m = requireType[model](t, next)
	requireEqual(
		t,
		cmd != nil,
		true,
		valueFormat,
		"opening comments did not request a refresh",
	)
	m = updateModel(t, m, cmd())
	requireEqual(
		t,
		m.mode == modeComments && len(m.comments.items) == zero, true,
		"mode=%v comments=%#v",
		m.mode,
		m.comments.items,
	)
}

func TestCommentReloadFailurePreservesCurrentComments(t *testing.T) {
	m := testModel(
		coveragePatch(),
		[]comments.Comment{{ID: "keep", Body: "keep"}},
		nil,
	)
	m.setLoadComments(
		func() ([]comments.Comment, error) {
			return nil, errors.New("storage unavailable")
		},
	)

	next, cmd := m.Update(textKey(commentsKey))
	m = requireType[model](t, next)
	m = updateModel(t, m, cmd())
	requireEqual(
		t,
		len(m.comments.items) == one && m.err != nil &&
			m.err.Error() == "refresh comments: storage unavailable",
		true,
		"comments=%#v error=%v",
		m.comments.items,
		m.err,
	)
}

func TestEmptyEditedCommentIsDeleted(t *testing.T) {
	t.Setenv(editorVariable, fixtureTrue)
	m := testModel(
		coveragePatch(),
		[]comments.Comment{{ID: "one", Body: "old"}},
		nil,
	)
	deleted := false
	m.setDelete(
		func(comments.Comment, patch.Patch) error {
			deleted = true
			return nil
		},
	)
	m = updateModel(t, m, textKey(commentsKey))
	m = updateModel(t, m, specialKey(tea.KeyEnter))
	m = updateModel(t, m, commentEditorFinishedMsg{body: newline})
	requireEqual(
		t,
		deleted && len(m.comments.items) == zero, true,
		"deleted=%v comments=%d",
		deleted,
		len(m.comments.items),
	)
}

func TestCommentDeleteFailureKeepsCommentAndShowsError(t *testing.T) {
	m := testModel(
		coveragePatch(),
		[]comments.Comment{{ID: "one", Body: "keep"}},
		nil,
	)
	m.setDelete(
		func(comments.Comment, patch.Patch) error {
			return errors.New("delete failed")
		},
	)
	m = updateModel(t, m, textKey(commentsKey))
	m = updateModel(t, m, textKey("D"))
	requireEqual(
		t,
		len(m.comments.items) == one &&
			strings.Contains(ansi.Strip(m.render()), "delete failed"),
		true,
		"comments=%#v render=%q",
		m.comments.items,
		m.render(),
	)
}

func TestSelectionCannotCrossHunk(t *testing.T) {
	m := testModel(coveragePatch(), nil, nil)
	m = updateModel(t, m, textKey(selectionKey))
	for range ten {
		m = updateModel(t, m, textKey(keyJ))
	}
	_, line, _ := m.diffView.Current()
	requireEqual(
		t,
		line.Text != "more()",
		true,
		valueFormat,
		"diffSelection crossed hunk",
	)
}

func TestVimSequencesAndLayoutToggle(t *testing.T) {
	m := testModel(coveragePatch(), nil, nil)
	var saved []bool
	m.configureSideBySide(
		false,
		func(enabled bool) error {
			saved = append(saved, enabled)
			return nil
		},
	)
	m = updateModel(t, m, tea.WindowSizeMsg{Width: 120, Height: 20})
	m = updateModel(t, m, textKey(fixtureG))
	requireEqual(t, lineText(m), "more()", "G line=%q", lineText(m))
	m = updateModel(t, m, textKey(keyG))
	m = updateModel(t, m, textKey(keyG))
	requireEqual(
		t,
		lineText(m), fixturePackageMain,
		"gg line=%q",
		lineText(m),
	)
	m = updateModel(t, m, textKey(fixtureT))
	requireEqual(
		t,
		m.diffOptions.SideBySide && strings.Contains(m.render(), "│"),
		true,
		valueFormat,
		"split view not enabled",
	)
	m = updateModel(t, m, textKey(fixtureT))
	requireEqual(
		t,
		!m.diffOptions.SideBySide && slices.Equal(saved, []bool{true, false}),
		true,
		"saved=%v",
		saved,
	)
}

func TestSavedSideBySideCanBeDisabledInNarrowTerminal(t *testing.T) {
	m := testModel(coveragePatch(), nil, nil)
	var saved []bool
	m.configureSideBySide(
		true,
		func(enabled bool) error {
			saved = append(saved, enabled)
			return nil
		},
	)
	m = updateModel(t, m, tea.WindowSizeMsg{Width: defaultWidth, Height: 20})
	m = updateModel(t, m, textKey(fixtureT))
	requireEqual(
		t,
		!m.diffOptions.SideBySide && slices.Equal(saved, []bool{false}),
		true,
		"sideBySide=%v saved=%v",
		m.diffOptions.SideBySide,
		saved,
	)
}

func TestResizeAcrossSideBySideThresholdPreservesFocusedLine(t *testing.T) {
	m := testModel(coveragePatch(), nil, nil)
	m.configureSideBySide(true, nil)
	focusLine(t, &m, fixtureKeep)
	_, before, _ := m.diffView.Current()
	m = updateModel(t, m, tea.WindowSizeMsg{Width: defaultWidth, Height: 20})
	m = updateModel(t, m, tea.WindowSizeMsg{Width: 120, Height: 20})
	_, got, ok := m.diffView.Current()
	requireEqual(
		t,
		ok && got == before, true,
		"focused line=%#v, want %#v",
		got,
		before,
	)
}

func TestZSequencesPositionCurrentLineInViewport(t *testing.T) {
	for _, test := range []struct {
		key       string
		alignment diffscreen.Alignment
	}{
		{"z", diffscreen.Center},
		{fixtureT, diffscreen.Top},
		{"b", diffscreen.Bottom},
	} {
		m := testModel(longModelPatch(), nil, nil)
		m = updateModel(t, m, tea.WindowSizeMsg{Width: hundred, Height: nine})
		want := diffscreen.New(
			longModelPatch(),
			diffscreen.Options{Dark: true},
		)
		want.Resize(hundred, nine)
		for range ten {
			m.diffView.Move(diffscreen.NextLine)
			want.Move(diffscreen.NextLine)
		}
		want.Align(test.alignment)
		m = updateModel(t, m, textKey("z"))
		m = updateModel(t, m, textKey(test.key))
		requireEqual(
			t,
			m.diffView.Render(nil), want.Render(nil),
			"z%s did not align view",
			test.key,
		)
	}
}

func TestPendingKeyIsConsumedByNextKey(t *testing.T) {
	m := testModel(coveragePatch(), nil, nil)
	m = updateModel(t, m, textKey(fixtureG))
	last := lineText(m)
	m = updateModel(t, m, textKey(keyG))
	m = updateModel(t, m, textKey(fixtureH))
	m = updateModel(t, m, textKey(keyG))
	requireEqual(
		t, lineText(m), last, valueFormat, "pending prefix moved focus",
	)
	m = updateModel(t, m, textKey(keyG))
	requireEqual(
		t,
		lineText(m), fixturePackageMain,
		"gg line=%q",
		lineText(m),
	)
}

func TestStatusShowsBasicBindingsAndHelpShowsCompleteKeyMap(t *testing.T) {
	m := testModel(coveragePatch(), nil, nil)
	status := ansi.Strip(strings.Split(m.render(), newline)[m.height-two])
	requireEqual(
		t,
		strings.HasPrefix(status, "j/k/h/l move") &&
			strings.HasSuffix(status, "local changes"),
		true,
		"status=%q",
		status,
	)
	help := ansi.Strip(m.renderHelp())
	for _, binding := range []string{
		"Ctrl-w h/l/w",
		"zz/zt/zb",
		"n/N",
		"]f/[f",
		"Tab",
		fixtureT,
	} {
		requireEqual(
			t,
			strings.Contains(help, binding), true,
			"help missing %q",
			binding,
		)
	}
}

func TestStatusShowsProgressOnlyAfterViewportMoves(t *testing.T) {
	m := testModel(longModelPatch(), nil, nil)
	m = updateModel(
		t,
		m,
		tea.WindowSizeMsg{Width: defaultWidth, Height: nine},
	)
	requireEqual(
		t,
		!strings.Contains(m.render(), "%"),
		true,
		valueFormat,
		"initial view shows progress",
	)
	m = updateModel(t, m, textKey("l"))
	requireEqual(
		t,
		!strings.Contains(m.render(), "%"),
		true,
		valueFormat,
		"horizontal scroll shows progress",
	)
	for range ten {
		m = updateModel(t, m, textKey(keyJ))
	}
	requireEqual(
		t,
		strings.Contains(m.render(), progressSuffix),
		true,
		valueFormat,
		"scrolled view hides progress",
	)
	m = updateModel(t, m, textKey(fixtureG))
	requireEqual(
		t,
		strings.Contains(m.render(), "local changes (100%)"),
		true,
		valueFormat,
		"final view hides progress",
	)
}

func TestStatusHidesProgressWhenDiffFitsViewport(t *testing.T) {
	m := testModel(coveragePatch(), nil, nil)
	m = updateModel(t, m, tea.WindowSizeMsg{Width: hundred, Height: hundred})
	requireEqual(
		t,
		!strings.Contains(m.render(), progressSuffix),
		true,
		valueFormat,
		"fitting diff shows progress",
	)
}

func TestSideBySidePaneSwitchingUsesCtrlWSequences(t *testing.T) {
	m := testModel(coveragePatch(), nil, nil)
	m = updateModel(
		t,
		m,
		tea.WindowSizeMsg{Width: 120, Height: defaultHeight},
	)
	m.setSideBySide(true)
	focusLine(t, &m, fixtureNew)
	m = updateModel(t, m, controlKey('w'))
	m = updateModel(t, m, textKey(fixtureH))
	requireEqual(t, lineText(m), fixtureOld, "old pane line=%q", lineText(m))
	m = updateModel(t, m, controlKey('w'))
	m = updateModel(t, m, controlKey('w'))
	requireEqual(t, lineText(m), fixtureNew, "new pane line=%q", lineText(m))
}

func TestHorizontalScrollKeysMoveByStepAndReset(t *testing.T) {
	m := testModel(longModelPatch(), nil, nil)
	m = updateModel(
		t,
		m,
		tea.WindowSizeMsg{Width: fixtureNumber37, Height: defaultHeight},
	)
	initial := m.diffView.Render(nil)
	want := diffscreen.New(longModelPatch(), diffscreen.Options{Dark: true})
	want.Resize(fixtureNumber37, defaultHeight)
	want.ScrollHorizontal(two * horizontalScrollStep)
	m = updateModel(t, m, textKey("l"))
	m = updateModel(t, m, tea.KeyPressMsg(tea.Key{Code: tea.KeyRight}))
	requireEqual(
		t,
		m.diffView.Render(nil), want.Render(nil),
		valueFormat,
		"right keys did not scroll by two steps",
	)
	m = updateModel(t, m, textKey(fixtureH))
	m = updateModel(t, m, tea.KeyPressMsg(tea.Key{Code: tea.KeyLeft}))
	requireEqual(
		t,
		m.diffView.Render(nil), initial,
		valueFormat,
		"left keys did not restore initial offset",
	)
	m = updateModel(t, m, textKey("$"))
	requireEqual(
		t,
		m.diffView.Render(nil) != initial,
		true,
		valueFormat,
		"$ did not scroll",
	)
	m = updateModel(t, m, textKey("0"))
	requireEqual(
		t,
		m.diffView.Render(nil), initial,
		valueFormat,
		"0 did not restore initial offset",
	)
}

func TestFocusAndManualRefreshLoadCurrentView(t *testing.T) {
	m := testModel(coveragePatch(), nil, nil)
	m.currentPatch.Branch = "main"
	m.kind = patch.Branch
	var requested []patch.Kind
	m.setRefresh(
		func(kind patch.Kind) (patch.Patch, error) {
			requested = append(requested, kind)
			p := coveragePatch()
			p.Kind, p.Branch = kind, "main"
			p.Files[zero].Metadata = []string{
				fmt.Sprintf("refresh-%d", len(requested)),
			}
			return p, nil
		},
	)
	next, cmd := m.Update(tea.FocusMsg{})
	m = requireType[model](t, next)
	requireEqual(t, cmd != nil, true, valueFormat, "focus did not refresh")
	m = updateModel(t, m, cmd())
	next, cmd = m.Update(textKey("R"))
	m = requireType[model](t, next)
	requireEqual(t, cmd != nil, true, valueFormat, "R did not refresh")
	m = updateModel(t, m, cmd())
	requireEqual(
		t,
		slices.Equal(requested, []patch.Kind{patch.Branch, patch.Branch}) &&
			slices.Equal(
				m.currentPatch.Files[zero].Metadata,
				[]string{"refresh-2"},
			),
		true,
		"requested=%v metadata=%q",
		requested,
		m.currentPatch.Files[zero].Metadata,
	)
}

func TestSourceEditorCompletionRefreshesDiff(t *testing.T) {
	m := testModel(coveragePatch(), nil, nil)
	refreshed := coveragePatch()
	refreshed.Files[zero].Metadata = []string{"after-editor"}
	m.setRefresh(
		func(patch.Kind) (patch.Patch, error) {
			return refreshed, nil
		},
	)

	next, cmd := m.Update(sourceEditorFinishedMsg{})
	m = requireType[model](t, next)
	requireEqual(
		t,
		cmd != nil,
		true,
		valueFormat,
		"editor completion did not refresh",
	)
	m = updateModel(t, m, cmd())
	requireEqual(
		t,
		slices.Equal(
			m.currentPatch.Files[zero].Metadata,
			[]string{"after-editor"},
		),
		true,
		"metadata=%q",
		m.currentPatch.Files[zero].Metadata,
	)
}

func TestHeaderShowsAddedAndRemovedLineCounts(t *testing.T) {
	header := strings.SplitN(
		ansi.Strip(testModel(coveragePatch(), nil, nil).render()),
		newline,
		two,
	)[zero]
	requireEqual(t, header, "review-my-slop  +2-1", "header = %q", header)
}

func TestSearchMovesIncrementallyRepeatsAndRestoresOrigin(t *testing.T) {
	m := testModel(coveragePatch(), nil, nil)
	origin := screenBody(m.diffView.Render(nil))
	m = updateModel(t, m, textKey(searchKey))
	m = updateModel(t, m, textKey("keep"))
	first := screenBody(m.diffView.Render(nil))
	requireEqual(
		t,
		m.mode == modeSearch && lineText(m) == fixtureKeep, true,
		"mode=%v line=%q",
		m.mode,
		lineText(m),
	)
	m = updateModel(t, m, specialKey(tea.KeyEnter))
	m = updateModel(t, m, textKey("n"))
	requireEqual(
		t,
		screenBody(m.diffView.Render(nil)) != first &&
			lineText(m) == fixtureKeep,
		true,
		"next=%#v",
		screenBody(m.diffView.Render(nil)),
	)
	m = updateModel(t, m, textKey("N"))
	requireEqual(
		t,
		screenBody(m.diffView.Render(nil)), first,
		"previous=%#v",
		screenBody(m.diffView.Render(nil)),
	)
	m = updateModel(t, m, textKey(searchKey))
	m = updateModel(t, m, textKey("missing"))
	requireEqual(
		t,
		strings.Contains(m.render(), "no matches"),
		true,
		valueFormat,
		"missing search did not miss",
	)
	m = updateModel(t, m, specialKey(tea.KeyEsc))
	requireEqual(
		t,
		screenBody(m.diffView.Render(nil)), first,
		"cancel=%#v origin=%#v",
		screenBody(m.diffView.Render(nil)),
		origin,
	)
}

func TestSearchMatchesFileNamesAndBackspaceRestoresOrigin(t *testing.T) {
	m := testModel(coveragePatch(), nil, nil)
	origin := screenBody(m.diffView.Render(nil))
	m = updateModel(t, m, textKey(searchKey))
	m = updateModel(t, m, textKey(fixtureMainGo))
	file, _, _ := m.diffView.Current()
	requireEqual(
		t,
		file.DisplayPath, fixtureMainGo,
		"file=%q",
		file.DisplayPath,
	)
	for range len(fixtureMainGo) {
		m = updateModel(t, m, specialKey(tea.KeyBackspace))
	}
	requireEqual(
		t,
		screenBody(m.diffView.Render(nil)), origin,
		"diffCursor=%#v",
		screenBody(m.diffView.Render(nil)),
	)
}

func TestSideBySideSearchActivatesPaneAndCancelRestoresIt(t *testing.T) {
	m := testModel(coveragePatch(), nil, nil)
	m = updateModel(
		t,
		m,
		tea.WindowSizeMsg{Width: 120, Height: defaultHeight},
	)
	m.setSideBySide(true)
	origin := screenBody(m.diffView.Render(nil))
	m = updateModel(t, m, textKey(searchKey))
	m = updateModel(t, m, textKey(fixtureOld))
	requireEqual(
		t,
		lineText(m), fixtureOld,
		"diffCursor=%#v line=%q",
		screenBody(m.diffView.Render(nil)),
		lineText(m),
	)
	m = updateModel(t, m, specialKey(tea.KeyEsc))
	requireEqual(
		t,
		screenBody(m.diffView.Render(nil)), origin,
		"cancel=%#v want=%#v",
		screenBody(m.diffView.Render(nil)),
		origin,
	)
}

func TestTabTogglesDefaultBranchAndIgnoresStaleRefresh(t *testing.T) {
	m := testModel(coveragePatch(), nil, nil)
	m.currentPatch.Branch = "main"
	m.setRefresh(func(kind patch.Kind) (patch.Patch, error) {
		p := coveragePatch()
		p.Kind, p.Branch = kind, "main"
		return p, nil
	})
	next, _ := m.Update(textKey(fixtureTab))
	m = requireType[model](t, next)
	requireEqual(t, m.kind, patch.Branch, "kind=%v", m.kind)
	stale := coveragePatch()
	stale.Files[zero].Metadata = []string{"stale"}
	m = updateModel(t, m, refreshDiffMsg{patch: stale})
	requireEqual(
		t,
		!slices.Equal(m.currentPatch.Files[zero].Metadata, []string{"stale"}),
		true,
		valueFormat,
		"stale refresh applied",
	)
	m = updateModel(t, m, textKey(fixtureTab))
	requireEqual(
		t,
		m.kind, patch.Unstaged,
		"kind=%v after toggling back to local",
		m.kind,
	)
}

func TestTabDoesNothingWithoutDefaultBranch(t *testing.T) {
	m := testModel(coveragePatch(), nil, nil)
	refreshed := false
	m.setRefresh(func(patch.Kind) (patch.Patch, error) {
		refreshed = true
		return coveragePatch(), nil
	})
	next, cmd := m.Update(textKey(fixtureTab))
	m = requireType[model](t, next)
	requireEqual(
		t,
		cmd == nil && !refreshed && m.kind == patch.Unstaged, true,
		"tab changed model without default branch"+
			": cmd=%v refreshed=%v kind=%v",
		cmd != nil,
		refreshed,
		m.kind,
	)
}

func TestDiffRefreshFallbackAndEmptyDiff(t *testing.T) {
	m := testModel(patch.Patch{}, nil, nil)
	m = updateModel(t, m, refreshDiffMsg{patch: coveragePatch()})
	requireEqual(
		t,
		lineText(m), fixturePackageMain,
		"initial focus=%q",
		lineText(m),
	)
	focusLine(t, &m, fixtureNew)
	changed := coveragePatch()
	changed.Files[zero].Hunks[zero].Lines[two].Text = "different()"
	m = updateModel(t, m, refreshDiffMsg{patch: changed})
	{
		_, _, ok := m.diffView.Current()
		requireEqual(t, ok, true, valueFormat, "refresh lost focus")
	}
}

func TestCommentAfterRefreshUsesCurrentPatch(t *testing.T) {
	t.Setenv(editorVariable, fixtureTrue)
	var saved patch.Patch
	m := testModel(
		coveragePatch(),
		nil,
		func(
			stored comments.Comment,
			p patch.Patch,
		) (
			comments.Comment,
			error,
		) {
			saved = p
			return stored, nil
		},
	)
	refreshed := coveragePatch()
	refreshed.Files[zero].Metadata = []string{"refreshed"}
	m = updateModel(t, m, refreshDiffMsg{patch: refreshed})
	m = updateModel(t, m, textKey(commentKey))
	_ = updateModel(t, m, commentEditorFinishedMsg{body: "comment"})
	requireEqual(
		t,
		slices.Equal(saved.Files[zero].Metadata, []string{"refreshed"}),
		true,
		"metadata=%q",
		saved.Files[zero].Metadata,
	)
}

func TestViewPreservesTerminalColors(t *testing.T) {
	result := testModel(coveragePatch(), nil, nil).View()
	requireEqual(
		t,
		result.BackgroundColor == nil && result.ForegroundColor == nil &&
			result.AltScreen &&
			result.ReportFocus,
		true,
		"view=%#v",
		result,
	)
	requireEqual(
		t,
		result.MouseMode, tea.MouseModeCellMotion,
		valueFormat,
		"view does not capture terminal wheel events",
	)
}

func TestMouseWheelScrollsContentWithoutMovingFocus(t *testing.T) {
	m := testModel(longModelPatch(), nil, nil)
	m = updateModel(
		t,
		m,
		tea.WindowSizeMsg{Width: fixtureNumber37, Height: nine},
	)
	m = updateModel(t, m, textKey(selectionKey))
	m = updateModel(t, m, textKey(keyJ))
	_, focus, _ := m.diffView.Current()
	_, selected, _ := m.diffView.Selected()
	initial := m.render()
	for _, button := range []tea.MouseButton{
		tea.MouseWheelDown,
		tea.MouseWheelRight,
	} {
		m = updateModel(t, m, tea.MouseWheelMsg{Button: button})
	}
	requireEqual(
		t,
		m.render() != initial && strings.Contains(m.render(), progressSuffix),
		true,
		valueFormat,
		"wheel did not scroll content",
	)
	_, got, _ := m.diffView.Current()
	_, gotSelected, _ := m.diffView.Selected()
	requireEqual(
		t,
		got == focus && slices.Equal(selected, gotSelected),
		true,
		valueFormat,
		"wheel changed focus or selection",
	)
	for _, button := range []tea.MouseButton{
		tea.MouseWheelUp,
		tea.MouseWheelLeft,
	} {
		m = updateModel(t, m, tea.MouseWheelMsg{Button: button})
	}
	requireEqual(
		t,
		m.render(), initial,
		valueFormat,
		"reverse wheel did not restore content",
	)
	for _, mode := range []mode{modeComments, modeHelp} {
		m.mode = mode
		before := m.diffView.Render(nil)
		m = updateModel(t, m, tea.MouseWheelMsg{Button: tea.MouseWheelDown})
		requireEqual(
			t,
			m.diffView.Render(nil), before,
			valueFormat,
			"wheel scrolled diff behind another screen",
		)
	}
}

func updateModel(t *testing.T, m model, msg tea.Msg) model {
	t.Helper()
	next, _ := m.Update(msg)
	result, ok := next.(model)
	requireEqual(t, ok, true, "model=%T", next)
	return result
}
func textKey(text string) tea.KeyPressMsg {
	runes := []rune(text)
	return tea.KeyPressMsg(tea.Key{Text: text, Code: runes[zero]})
}
func specialKey(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Code: code})
}
func controlKey(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Code: code, Mod: tea.ModCtrl})
}

func focusLine(t *testing.T, m *model, text string) {
	t.Helper()
	m.diffView.Move(diffscreen.FirstLine)
	count := zero
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

func lineText(m model) string {
	_, line, _ := m.diffView.Current()
	return line.Text
}

func screenBody(rendered string) string {
	lines := strings.Split(rendered, newline)
	return strings.Join(lines[one:len(lines)-two], newline)
}

func coveragePatch() patch.Patch {
	return patch.Patch{
		Root: "/repo",
		Files: []patch.File{
			{
				DisplayPath: fixtureMainGo,
				OldPath:     fixtureMainGo,
				NewPath:     fixtureMainGo,
				OldSource:   "package main\nold()\nkeep()\n",
				NewSource:   "package main\nnew()\nkeep()\nmore()\n",
				Hunks: []patch.Hunk{
					{
						Header: "@@ -1,3 +1,3 @@",
						Lines: []patch.Line{
							{
								Kind:      patch.Context,
								Text:      fixturePackageMain,
								OldNumber: one,
								NewNumber: one,
							},
							{
								Kind:      patch.Deletion,
								Text:      fixtureOld,
								OldNumber: two,
							},
							{
								Kind:      patch.Addition,
								Text:      fixtureNew,
								NewNumber: two,
							},
							{
								Kind:      patch.Context,
								Text:      fixtureKeep,
								OldNumber: three,
								NewNumber: three,
							},
						},
					},

					{
						Header: "@@ -3,1 +3,2 @@",
						Lines: []patch.Line{
							{
								Kind:      patch.Context,
								Text:      fixtureKeep,
								OldNumber: three,
								NewNumber: three,
							},
							{
								Kind:      patch.Addition,
								Text:      "more()",
								NewNumber: four,
							},
						},
					},
				},
			},
		},
	}
}

func longModelPatch() patch.Patch {
	lines := make([]patch.Line, defaultHeight)
	for index := range lines {
		lines[index] = patch.Line{
			Kind: patch.Context,
			Text: fmt.Sprintf(
				"line %d %s",
				index,
				strings.Repeat("x", defaultWidth),
			),
			OldNumber: patch.LineNumber(index + one),
			NewNumber: patch.LineNumber(index + one),
		}
	}
	return patch.Patch{
		Files: []patch.File{
			{
				DisplayPath: "long.go",
				Hunks:       []patch.Hunk{{Header: "@@", Lines: lines}},
			},
		},
	}
}
