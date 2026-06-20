package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/eskelinenantti/review-my-slop/internal/editor"
	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/review"
	"github.com/eskelinenantti/review-my-slop/internal/view"
)

func TestVisualSelectionCreatesMappedAnchorAndSubmits(t *testing.T) {
	t.Setenv("EDITOR", "true")
	var saved []review.Comment
	m := New(coveragePatch(), nil, func(stored review.Comment, _ patch.Patch) (review.Comment, error) {
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
	m := New(coveragePatch(), nil, func(review.Comment, patch.Patch) (review.Comment, error) {
		return review.Comment{}, fmt.Errorf("storage unavailable")
	})
	m = updateModel(t, m, textKey("c"))
	m = updateModel(t, m, commentEditorFinishedMsg{body: "keep this"})
	if m.commentBody != "" || m.err == nil || m.err.Error() != "storage unavailable" {
		t.Fatalf("body=%q err=%v", m.commentBody, m.err)
	}
}

func TestCommentRequiresEditor(t *testing.T) {
	t.Setenv("EDITOR", "")
	m := updateModel(t, New(coveragePatch(), nil, nil), textKey("c"))
	if m.err == nil || m.err.Error() != "$EDITOR is not set" {
		t.Fatalf("error = %v", m.err)
	}
}

func TestCommentOpensMarkdownFileInEditor(t *testing.T) {
	state := t.TempDir()
	t.Setenv("XDG_STATE_HOME", state)
	anchor := review.Anchor{QuotedLines: []string{"-old()```x", "+new()"}}
	path, err := editor.CreateCommentFile("existing comment", anchor)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(path) })
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Ext(path) != ".md" || info.Mode().Perm() != 0o600 || string(body) != "existing comment\n\n```suggestion\nnew()\n```\n" {
		t.Fatalf("path=%q mode=%o body=%q", path, info.Mode().Perm(), body)
	}
}

func TestCommentEditorSuggestionBehaviors(t *testing.T) {
	t.Run("escapes fence", func(t *testing.T) {
		anchor := review.Anchor{QuotedLines: []string{"+````go", `+fmt.Println("hello")`, "+````"}}
		draft := editor.CommentDraft("explain this", anchor)
		if !strings.Contains(draft, "`````suggestion") || editor.StripUnchangedSuggestion(draft, anchor.QuotedLines) != "explain this" {
			t.Fatalf("draft = %q", draft)
		}
	})
	t.Run("only new version", func(t *testing.T) {
		anchor := review.Anchor{QuotedLines: []string{" unchanged()", "-old()", "+new()"}}
		if got := editor.CommentDraft("comment", anchor); got != "comment\n\n```suggestion\nunchanged()\nnew()\n```\n" {
			t.Fatalf("draft = %q", got)
		}
	})
	t.Run("edited suggestion remains", func(t *testing.T) {
		body := "comment\n\n```suggestion\nbetter()\n```\n"
		if got := editor.StripUnchangedSuggestion(body, []string{"-old()", "+new()"}); got != body {
			t.Fatalf("body = %q", got)
		}
	})
}

func TestExternalEditorCommandReadsEditedDraft(t *testing.T) {
	file, err := os.CreateTemp("", "review-my-slop-editor-test-*.md")
	if err != nil {
		t.Fatal(err)
	}
	path := file.Name()
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(path) })
	if err := editor.CommentCommand("printf 'edited externally' >", path).Run(); err != nil {
		t.Fatal(err)
	}
	body, err := editor.ReadCommentFile(path, review.Anchor{}, nil)
	if err != nil || body != "edited externally" {
		t.Fatalf("body = %q, err = %v", body, err)
	}
}

func TestEmptyNewCommentIsDiscarded(t *testing.T) {
	t.Setenv("EDITOR", "true")
	called := false
	m := New(coveragePatch(), nil, func(stored review.Comment, p patch.Patch) (review.Comment, error) {
		called = true
		return stored, nil
	})
	m = updateModel(t, m, textKey("c"))
	m = updateModel(t, m, commentEditorFinishedMsg{body: " \n"})
	if called || len(m.comments) != 0 {
		t.Fatalf("called=%v comments=%d", called, len(m.comments))
	}
}

func TestOpenCurrentLineUsesEditorWithWorkingTreeLocation(t *testing.T) {
	t.Setenv("EDITOR", "printf")
	m := New(coveragePatch(), nil, nil)
	m.patch.Repository = "/tmp/repo with spaces"
	m.cursor = findLine(t, m, "new()")
	cmd, err := m.openCurrentLine()
	if err != nil || cmd == nil {
		t.Fatalf("command=%v err=%v", cmd, err)
	}
	want := "sh\x00-c\x00printf +2 '/tmp/repo with spaces/main.go'"
	if got := strings.Join(editor.SourceCommand("printf", "/tmp/repo with spaces/main.go", 2).Args, "\x00"); got != want {
		t.Fatalf("command = %q", got)
	}
}

func TestOpenCurrentLineRequiresEditor(t *testing.T) {
	t.Setenv("EDITOR", "")
	m := updateModel(t, New(coveragePatch(), nil, nil), textKey("e"))
	if m.err == nil || m.err.Error() != "$EDITOR is not set" {
		t.Fatalf("error = %v", m.err)
	}
}

func TestInboxCommentsCanBeViewedEditedAndDeleted(t *testing.T) {
	t.Setenv("EDITOR", "true")
	comments := []review.Comment{{ID: "one", Body: "old body"}, {ID: "two", Body: "second"}}
	var persisted, deleted review.Comment
	m := New(coveragePatch(), comments, func(stored review.Comment, _ patch.Patch) (review.Comment, error) {
		persisted = stored
		return stored, nil
	})
	m.SetDelete(func(stored review.Comment, _ patch.Patch) error { deleted = stored; return nil })
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
	if deleted.ID != "one" || len(m.comments) != 1 {
		t.Fatalf("deleted=%#v comments=%#v", deleted, m.comments)
	}
	m = updateModel(t, m, textKey("q"))
	if m.mode != modeBrowse || m.quitting {
		t.Fatalf("mode=%v quitting=%v", m.mode, m.quitting)
	}
}

func TestEmptyEditedCommentIsDeleted(t *testing.T) {
	t.Setenv("EDITOR", "true")
	m := New(coveragePatch(), []review.Comment{{ID: "one", Body: "old"}}, nil)
	deleted := false
	m.SetDelete(func(review.Comment, patch.Patch) error { deleted = true; return nil })
	m = updateModel(t, m, textKey("C"))
	m = updateModel(t, m, specialKey(tea.KeyEnter))
	m = updateModel(t, m, commentEditorFinishedMsg{body: "\n"})
	if !deleted || len(m.comments) != 0 {
		t.Fatalf("deleted=%v comments=%d", deleted, len(m.comments))
	}
}

func TestCommentDeleteFailureKeepsCommentAndShowsError(t *testing.T) {
	m := New(coveragePatch(), []review.Comment{{ID: "one", Body: "keep"}}, nil)
	m.SetDelete(func(review.Comment, patch.Patch) error { return fmt.Errorf("delete failed") })
	m = updateModel(t, m, textKey("C"))
	m = updateModel(t, m, textKey("D"))
	if len(m.comments) != 1 || !strings.Contains(ansi.Strip(m.renderComments()), "delete failed") {
		t.Fatalf("comments=%#v render=%q", m.comments, m.renderComments())
	}
}

func TestSelectionCannotCrossHunk(t *testing.T) {
	m := New(coveragePatch(), nil, nil)
	m = updateModel(t, m, textKey("v"))
	for range 10 {
		m = updateModel(t, m, textKey("j"))
	}
	line, _ := m.view.Line(m.cursor)
	if line.Text == "more()" {
		t.Fatal("selection crossed hunk")
	}
}

func TestVimSequencesAndLayoutToggle(t *testing.T) {
	m := New(coveragePatch(), nil, nil)
	var saved []bool
	m.SetSideBySide(false, func(enabled bool) error { saved = append(saved, enabled); return nil })
	m = updateModel(t, m, tea.WindowSizeMsg{Width: 120, Height: 20})
	m = updateModel(t, m, textKey("G"))
	last, _ := m.view.Last()
	if m.cursor != last {
		t.Fatalf("G cursor = %#v", m.cursor)
	}
	m = updateModel(t, m, textKey("g"))
	m = updateModel(t, m, textKey("g"))
	first, _ := m.view.First()
	if m.cursor != first {
		t.Fatalf("gg cursor = %#v", m.cursor)
	}
	m = updateModel(t, m, textKey("t"))
	if !m.sideBySide || !strings.Contains(m.render(), "│") {
		t.Fatal("split view not enabled")
	}
	m = updateModel(t, m, textKey("t"))
	if m.sideBySide || !slices.Equal(saved, []bool{true, false}) {
		t.Fatalf("sideBySide=%v saved=%v", m.sideBySide, saved)
	}
}

func TestSavedSideBySideCanBeDisabledInNarrowTerminal(t *testing.T) {
	m := New(coveragePatch(), nil, nil)
	var saved []bool
	m.SetSideBySide(true, func(enabled bool) error { saved = append(saved, enabled); return nil })
	m = updateModel(t, m, tea.WindowSizeMsg{Width: 80, Height: 20})
	m = updateModel(t, m, textKey("t"))
	if m.sideBySide || !slices.Equal(saved, []bool{false}) {
		t.Fatalf("sideBySide=%v saved=%v", m.sideBySide, saved)
	}
}

func TestResizeAcrossSideBySideThresholdPreservesCursorScreenRow(t *testing.T) {
	m := New(coveragePatch(), nil, nil)
	m.SetSideBySide(true, nil)
	m.cursor = findLine(t, m, "keep()")
	m.viewport = m.view.Align(m.viewport, m.cursor, view.Middle)
	before := m.cursor.Coordinate.Y - m.viewport.Top.Y
	m = updateModel(t, m, tea.WindowSizeMsg{Width: 80, Height: 20})
	m = updateModel(t, m, tea.WindowSizeMsg{Width: 120, Height: 20})
	if got := m.cursor.Coordinate.Y - m.viewport.Top.Y; got != before {
		t.Fatalf("screen row = %d, want %d", got, before)
	}
}

func TestZSequencesPositionCurrentLineInViewport(t *testing.T) {
	m := New(longModelPatch(), nil, nil)
	for range 10 {
		m.move(view.Forward)
	}
	m.height = 9
	m.viewport = m.view.Resize(m.viewport, m.width, m.viewportHeight())
	for _, test := range []struct {
		key       string
		alignment view.VerticalAlignment
	}{{"z", view.Middle}, {"t", view.Top}, {"b", view.Bottom}} {
		m = updateModel(t, m, textKey("z"))
		m = updateModel(t, m, textKey(test.key))
		want := m.view.Align(m.viewport, m.cursor, test.alignment)
		if m.viewport.Top != want.Top {
			t.Errorf("z%s top=%v want=%v", test.key, m.viewport.Top, want.Top)
		}
	}
}

func TestPendingKeyIsConsumedByNextKey(t *testing.T) {
	m := New(coveragePatch(), nil, nil)
	m = updateModel(t, m, textKey("G"))
	last := m.cursor
	m = updateModel(t, m, textKey("g"))
	m = updateModel(t, m, textKey("h"))
	m = updateModel(t, m, textKey("g"))
	if m.cursor != last || m.pendingKey != "g" {
		t.Fatalf("cursor=%#v pending=%q", m.cursor, m.pendingKey)
	}
}

func TestStatusShowsBasicBindingsAndHelpShowsCompleteKeyMap(t *testing.T) {
	m := New(coveragePatch(), nil, nil)
	status := ansi.Strip(m.renderStatus())
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
	m := New(longModelPatch(), nil, nil)
	m = updateModel(t, m, tea.WindowSizeMsg{Width: 80, Height: 9})
	if label := m.viewLabel(); label != "local changes" {
		t.Fatalf("initial label=%q", label)
	}
	m = updateModel(t, m, textKey("l"))
	if label := m.viewLabel(); label != "local changes" {
		t.Fatalf("horizontal-scroll label=%q", label)
	}
	for m.viewport.Top.Y == 0 {
		m = updateModel(t, m, textKey("j"))
	}
	if label := m.viewLabel(); !strings.HasPrefix(label, "local changes (") || !strings.HasSuffix(label, "%)") {
		t.Fatalf("scrolled label=%q", label)
	}
	m = updateModel(t, m, textKey("G"))
	if label := m.viewLabel(); label != "local changes (100%)" {
		t.Fatalf("final label=%q", label)
	}
}

func TestStatusHidesProgressWhenDiffFitsViewport(t *testing.T) {
	m := New(coveragePatch(), nil, nil)
	m = updateModel(t, m, tea.WindowSizeMsg{Width: 100, Height: 100})
	if label := m.viewLabel(); label != "local changes" {
		t.Fatalf("label=%q", label)
	}
}

func TestRenderKeyBindingsAlignsDescriptions(t *testing.T) {
	lines := renderKeyBindings([]keyBinding{{keys: "x", description: "short"}, {keys: "long", description: "wide"}})
	if strings.Index(lines[0], "short") != strings.Index(lines[1], "wide") {
		t.Fatalf("lines are not aligned: %#v", lines)
	}
}

func TestSideBySidePaneSwitchingUsesCtrlWSequences(t *testing.T) {
	m := New(coveragePatch(), nil, nil)
	m.width = 120
	m.setSideBySide(true)
	m.cursor = findLine(t, m, "new()")
	m = updateModel(t, m, controlKey('w'))
	m = updateModel(t, m, textKey("h"))
	if m.cursor.Pane != view.Left || lineText(m) != "old()" {
		t.Fatalf("left cursor=%#v line=%q", m.cursor, lineText(m))
	}
	m = updateModel(t, m, controlKey('w'))
	m = updateModel(t, m, controlKey('w'))
	if m.cursor.Pane != view.Right || lineText(m) != "new()" {
		t.Fatalf("right cursor=%#v line=%q", m.cursor, lineText(m))
	}
}

func TestHorizontalScrollKeysMoveByStepAndReset(t *testing.T) {
	m := New(longModelPatch(), nil, nil)
	m.width = 37
	m.viewport = m.view.Resize(m.viewport, m.width, m.viewportHeight())
	m = updateModel(t, m, textKey("l"))
	m = updateModel(t, m, tea.KeyPressMsg(tea.Key{Code: tea.KeyRight}))
	if m.viewport.LeftColumn != 2*horizontalScrollStep {
		t.Fatalf("right offset=%d", m.viewport.LeftColumn)
	}
	m = updateModel(t, m, textKey("h"))
	m = updateModel(t, m, tea.KeyPressMsg(tea.Key{Code: tea.KeyLeft}))
	if m.viewport.LeftColumn != 0 {
		t.Fatalf("left offset=%d", m.viewport.LeftColumn)
	}
	m = updateModel(t, m, textKey("$"))
	if m.viewport.LeftColumn == 0 {
		t.Fatal("$ did not move to end")
	}
	m = updateModel(t, m, textKey("0"))
	if m.viewport.LeftColumn != 0 {
		t.Fatalf("0 offset=%d", m.viewport.LeftColumn)
	}
}

func TestFocusAndManualRefreshLoadCurrentView(t *testing.T) {
	m := New(coveragePatch(), nil, nil)
	m.SetParents([]string{"main"})
	m.target = 1
	var requested []string
	m.SetRefresh(func(parent string) (patch.Patch, error) {
		requested = append(requested, parent)
		p := coveragePatch()
		p.Fingerprint = fmt.Sprintf("refresh-%d", len(requested))
		return p, nil
	})
	next, cmd := m.Update(tea.FocusMsg{})
	m = next.(Model)
	if cmd == nil {
		t.Fatal("focus did not refresh")
	}
	m = updateModel(t, m, cmd())
	next, cmd = m.Update(textKey("R"))
	m = next.(Model)
	if cmd == nil {
		t.Fatal("R did not refresh")
	}
	m = updateModel(t, m, cmd())
	if !slices.Equal(requested, []string{"main", "main"}) || m.patch.Fingerprint != "refresh-2" {
		t.Fatalf("requested=%v fingerprint=%q", requested, m.patch.Fingerprint)
	}
}

func TestHeaderShowsAddedAndRemovedLineCounts(t *testing.T) {
	header := strings.SplitN(ansi.Strip(New(coveragePatch(), nil, nil).render()), "\n", 2)[0]
	if header != "review-my-slop  +2-1" {
		t.Fatalf("header = %q", header)
	}
}

func TestSearchMovesIncrementallyRepeatsAndRestoresOrigin(t *testing.T) {
	m := New(coveragePatch(), nil, nil)
	origin := m.cursor
	m = updateModel(t, m, textKey("/"))
	m = updateModel(t, m, textKey("keep"))
	first := m.cursor
	if m.mode != modeSearch || lineText(m) != "keep()" {
		t.Fatalf("mode=%v line=%q", m.mode, lineText(m))
	}
	m = updateModel(t, m, specialKey(tea.KeyEnter))
	m = updateModel(t, m, textKey("n"))
	if m.cursor == first || lineText(m) != "keep()" {
		t.Fatalf("next=%#v", m.cursor)
	}
	m = updateModel(t, m, textKey("N"))
	if m.cursor != first {
		t.Fatalf("previous=%#v", m.cursor)
	}
	m = updateModel(t, m, textKey("/"))
	m = updateModel(t, m, textKey("missing"))
	if !m.searchMiss {
		t.Fatal("missing search did not miss")
	}
	m = updateModel(t, m, specialKey(tea.KeyEsc))
	if m.cursor != first {
		t.Fatalf("cancel=%#v origin=%#v", m.cursor, origin)
	}
}

func TestSearchMatchesFileNamesAndBackspaceRestoresOrigin(t *testing.T) {
	m := New(coveragePatch(), nil, nil)
	origin := m.cursor
	m = updateModel(t, m, textKey("/"))
	m = updateModel(t, m, textKey("main.go"))
	file, _ := m.view.File(m.cursor)
	if file.DisplayPath != "main.go" {
		t.Fatalf("file=%q", file.DisplayPath)
	}
	for range len("main.go") {
		m = updateModel(t, m, specialKey(tea.KeyBackspace))
	}
	if m.cursor != origin || len(m.search) != 0 {
		t.Fatalf("cursor=%#v query=%q", m.cursor, m.search)
	}
}

func TestSideBySideSearchActivatesPaneAndCancelRestoresIt(t *testing.T) {
	m := New(coveragePatch(), nil, nil)
	m.width = 120
	m.setSideBySide(true)
	origin := m.cursor
	m = updateModel(t, m, textKey("/"))
	m = updateModel(t, m, textKey("old()"))
	if m.cursor.Pane != view.Left || lineText(m) != "old()" {
		t.Fatalf("cursor=%#v line=%q", m.cursor, lineText(m))
	}
	m = updateModel(t, m, specialKey(tea.KeyEsc))
	if m.cursor != origin {
		t.Fatalf("cancel=%#v want=%#v", m.cursor, origin)
	}
}

func TestTabCyclesParentBranchesAndIgnoresStaleRefresh(t *testing.T) {
	m := New(coveragePatch(), nil, nil)
	m.SetParents([]string{"main", "release"})
	m.SetRefresh(func(string) (patch.Patch, error) { return coveragePatch(), nil })
	next, _ := m.Update(textKey("tab"))
	m = next.(Model)
	if m.currentParent() != "main" {
		t.Fatalf("parent=%q", m.currentParent())
	}
	stale := coveragePatch()
	stale.Fingerprint = "stale"
	m = updateModel(t, m, refreshDiffMsg{patch: stale, parent: "release"})
	if m.patch.Fingerprint == "stale" {
		t.Fatal("stale refresh applied")
	}
}

func TestDiffRefreshFallbackAndEmptyDiff(t *testing.T) {
	m := New(patch.Patch{}, nil, nil)
	refreshed := coveragePatch()
	refreshed.Fingerprint = "new"
	m = updateModel(t, m, refreshDiffMsg{patch: refreshed})
	first, ok := m.view.First()
	if !ok || m.cursor != first {
		t.Fatalf("cursor=%#v first=%#v", m.cursor, first)
	}
	m.cursor = findLine(t, m, "new()")
	changed := coveragePatch()
	changed.Fingerprint = "changed"
	changed.Files[0].Hunks[0].Lines[2].Text = "different()"
	m = updateModel(t, m, refreshDiffMsg{patch: changed})
	if _, ok := m.view.Line(m.cursor); !ok {
		t.Fatal("refresh fallback lost cursor")
	}
}

func TestCommentAfterRefreshUsesCurrentPatch(t *testing.T) {
	t.Setenv("EDITOR", "true")
	var saved patch.Patch
	m := New(coveragePatch(), nil, func(stored review.Comment, p patch.Patch) (review.Comment, error) {
		saved = p
		return stored, nil
	})
	refreshed := coveragePatch()
	refreshed.Fingerprint = "refreshed"
	m = updateModel(t, m, refreshDiffMsg{patch: refreshed})
	m = updateModel(t, m, textKey("c"))
	_ = updateModel(t, m, commentEditorFinishedMsg{body: "comment"})
	if saved.Fingerprint != "refreshed" {
		t.Fatalf("fingerprint=%q", saved.Fingerprint)
	}
}

func TestViewPreservesTerminalColors(t *testing.T) {
	result := New(coveragePatch(), nil, nil).View()
	if result.BackgroundColor != nil || result.ForegroundColor != nil || !result.AltScreen || !result.ReportFocus {
		t.Fatalf("view=%#v", result)
	}
}

func updateModel(t *testing.T, m Model, msg tea.Msg) Model {
	t.Helper()
	next, _ := m.Update(msg)
	result, ok := next.(Model)
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

func findLine(t *testing.T, m Model, text string) view.Cursor {
	t.Helper()
	cursor, ok := m.view.First()
	if !ok {
		t.Fatal("no cursor")
	}
	for {
		line, _ := m.view.Line(cursor)
		if line.Text == text {
			return cursor
		}
		cursor, ok = m.view.Move(cursor, view.Forward)
		if !ok {
			break
		}
	}
	t.Fatalf("line %q not found", text)
	return view.Cursor{}
}
func lineText(m Model) string { line, _ := m.view.Line(m.cursor); return line.Text }

func coveragePatch() patch.Patch {
	return patch.Patch{Repository: "/repo", Fingerprint: "fingerprint", Files: []patch.File{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", OldSource: "package main\nold()\nkeep()\n", NewSource: "package main\nnew()\nkeep()\nmore()\n", Hunks: []patch.Hunk{
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
