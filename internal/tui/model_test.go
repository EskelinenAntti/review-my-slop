package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/anttieskelinen/review-my-slop/internal/review"
)

func TestVisualSelectionCreatesMappedAnchorAndSubmits(t *testing.T) {
	t.Setenv("EDITOR", "true")
	var saved []review.Comment
	model := New(testDiff(), nil, func(stored review.StoredComment, _ review.Diff) (review.StoredComment, error) {
		saved = append(saved, stored.Comment)
		stored.BatchID = "new"
		return stored, nil
	})

	model = update(t, model, textKey("j"))
	model = update(t, model, textKey("v"))
	model = update(t, model, textKey("j"))
	model = update(t, model, textKey("c"))
	_ = update(t, model, commentEditorFinishedMsg{body: "fix both lines"})
	if len(saved) != 1 {
		t.Fatalf("saved comments = %d, want 1", len(saved))
	}
	anchor := saved[0].Anchor
	if anchor.File != "main.go" || anchor.OldStart != 2 || anchor.OldEnd != 2 ||
		anchor.NewStart != 2 || anchor.NewEnd != 2 {
		t.Fatalf("unexpected anchor: %#v", anchor)
	}
	if len(anchor.QuotedLines) != 2 ||
		anchor.QuotedLines[0] != "-old()" ||
		anchor.QuotedLines[1] != "+new()" {
		t.Fatalf("unexpected quoted lines: %#v", anchor.QuotedLines)
	}

	if saved[0].Body != "fix both lines" {
		t.Fatalf("unexpected saved comment: %#v", saved[0])
	}
}

func TestCommentSaveFailureClearsPendingEdit(t *testing.T) {
	t.Setenv("EDITOR", "true")
	model := New(testDiff(), nil, func(review.StoredComment, review.Diff) (review.StoredComment, error) {
		return review.StoredComment{}, fmt.Errorf("storage unavailable")
	})
	model = update(t, model, textKey("c"))
	model = update(t, model, commentEditorFinishedMsg{body: "keep this"})

	if model.mode != modeBrowse || model.commentBody != "" {
		t.Fatalf("failed save retained edit state: mode=%v body=%q", model.mode, model.commentBody)
	}
	if model.err == nil || model.err.Error() != "storage unavailable" {
		t.Fatalf("error = %v, want storage failure", model.err)
	}
}

func TestCommentRequiresEditor(t *testing.T) {
	t.Setenv("EDITOR", "")
	model := New(testDiff(), nil, nil)
	model = update(t, model, textKey("c"))

	if model.err == nil || model.err.Error() != "$EDITOR is not set" {
		t.Fatalf("error = %v, want missing editor error", model.err)
	}
}

func TestCommentOpensMarkdownFileInEditor(t *testing.T) {
	state := t.TempDir()
	t.Setenv("XDG_STATE_HOME", state)
	anchor := review.Anchor{QuotedLines: []string{"-old()```x", "+new()"}}
	path, err := createCommentFile("existing comment", anchor)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(path)
	if filepath.Ext(path) != ".md" {
		t.Fatalf("comment path = %q, want .md extension", path)
	}
	if filepath.Dir(path) != filepath.Join(state, "review-my-slop") {
		t.Fatalf("comment directory = %q, want XDG state directory", filepath.Dir(path))
	}
	dirInfo, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if dirInfo.Mode().Perm() != 0o700 {
		t.Fatalf("state directory mode = %o, want 700", dirInfo.Mode().Perm())
	}
	fileInfo, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if fileInfo.Mode().Perm() != 0o600 {
		t.Fatalf("comment file mode = %o, want 600", fileInfo.Mode().Perm())
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(body); got != "existing comment\n\n```suggestion\nnew()\n```\n" {
		t.Fatalf("body = %q, want comment and selected-code suggestion", got)
	}
}

func TestCommentEditorEscapesSelectedMarkdownCodeFence(t *testing.T) {
	anchor := review.Anchor{
		QuotedLines: []string{"+````go", `+fmt.Println("hello")`, "+````"},
	}
	draft := commentEditorDraft("explain this", anchor)
	want := "explain this\n\n`````suggestion\n````go\nfmt.Println(\"hello\")\n````\n`````\n"
	if draft != want {
		t.Fatalf("draft = %q, want escaped Markdown code fence", draft)
	}
	if got := stripUnchangedSuggestion(draft, anchor.QuotedLines); got != "explain this" {
		t.Fatalf("body = %q, want unchanged escaped suggestion removed", got)
	}
}

func TestCommentEditorContextIsNotSaved(t *testing.T) {
	path, err := createCommentFile("fix this", review.Anchor{
		QuotedLines: []string{" unchanged()", "-old()", "+new()"},
	})
	if err != nil {
		t.Fatal(err)
	}

	anchor := review.Anchor{QuotedLines: []string{" unchanged()", "-old()", "+new()"}}
	msg := readCommentEditorResult(path, anchor, nil)
	if msg.err != nil {
		t.Fatal(msg.err)
	}
	if msg.body != "fix this" {
		t.Fatalf("body = %q, want selected-code context removed", msg.body)
	}
}

func TestCommentEditorKeepsEditedSuggestion(t *testing.T) {
	lines := []string{"-old()", "+new()"}
	body := "comment\n\n```suggestion\nbetter()\nnew()\n```\n"
	if got := stripUnchangedSuggestion(body, lines); got != body {
		t.Fatalf("body = %q, want edited suggestion preserved", got)
	}
}

func TestCommentEditorSuggestionContainsOnlyNewVersion(t *testing.T) {
	anchor := review.Anchor{
		QuotedLines: []string{" unchanged()", "-old()", "+new()"},
	}
	want := "comment\n\n```suggestion\nunchanged()\nnew()\n```\n"
	if got := commentEditorDraft("comment", anchor); got != want {
		t.Fatalf("draft = %q, want suggestion with only new-version code", got)
	}
}

func TestEmptyNewCommentIsDiscarded(t *testing.T) {
	t.Setenv("EDITOR", "true")
	called := false
	model := New(testDiff(), nil, func(stored review.StoredComment, diff review.Diff) (review.StoredComment, error) {
		called = true
		return stored, nil
	})
	model = update(t, model, textKey("c"))
	model = update(t, model, commentEditorFinishedMsg{body: " \n"})

	if called || model.mode != modeBrowse || len(model.comments) != 0 {
		t.Fatalf("called=%v mode=%v comments=%d, want discarded comment", called, model.mode, len(model.comments))
	}
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

	if err := commentEditorCommand("printf 'edited externally' >", path).Run(); err != nil {
		t.Fatal(err)
	}
	msg := readCommentEditorResult(path, review.Anchor{}, nil)
	if msg.err != nil {
		t.Fatal(msg.err)
	}
	if msg.body != "edited externally" {
		t.Fatalf("body = %q, want external editor contents", msg.body)
	}
}

func TestOpenCurrentLineUsesEditorWithWorkingTreeLocation(t *testing.T) {
	t.Setenv("EDITOR", "printf")
	model := New(testDiff(), nil, nil)
	model.diff.Repository = filepath.Join(string(filepath.Separator), "tmp", "repo with spaces")
	model.cursor = findCodeRow(t, model, review.LineAdded)

	cmd, err := model.openCurrentLine()
	if err != nil {
		t.Fatal(err)
	}
	if cmd == nil {
		t.Fatal("openCurrentLine returned no command")
	}

	command := sourceEditorCommand(os.Getenv("EDITOR"), filepath.Join(model.diff.Repository, "main.go"), 2)
	if got, want := strings.Join(command.Args, "\x00"), "sh\x00-c\x00printf +2 '/tmp/repo with spaces/main.go'"; got != want {
		t.Fatalf("command = %q, want %q", got, want)
	}
}

func TestOpenCurrentLineRequiresEditor(t *testing.T) {
	t.Setenv("EDITOR", "")
	model := New(testDiff(), nil, nil)
	model = update(t, model, textKey("e"))

	if model.err == nil || model.err.Error() != "$EDITOR is not set" {
		t.Fatalf("error = %v, want missing editor error", model.err)
	}
}

func TestInboxCommentsCanBeViewedAndEdited(t *testing.T) {
	t.Setenv("EDITOR", "true")
	comments := []review.StoredComment{{
		BatchID: "batch-1",
		Comment: review.Comment{
			Anchor: review.Anchor{File: "main.go", NewStart: 2},
			Body:   "old body",
		},
	}}
	var persisted review.StoredComment
	model := New(testDiff(), comments, func(stored review.StoredComment, _ review.Diff) (review.StoredComment, error) {
		persisted = stored
		return stored, nil
	})

	model = update(t, model, textKey("C"))
	if model.mode != modeComments || !strings.Contains(model.render(), "old body") {
		t.Fatal("inbox comments view did not open")
	}
	model = update(t, model, specialKey(tea.KeyEnter))
	model = update(t, model, commentEditorFinishedMsg{body: "edited body"})

	if persisted.BatchID != "batch-1" || persisted.Comment.Body != "edited body" {
		t.Fatalf("persisted = %#v, want edited existing comment", persisted)
	}
	if model.mode != modeComments || model.comments[0].Comment.Body != "edited body" {
		t.Fatal("edited comment was not reflected in inbox view")
	}
}

func TestQReturnsFromCommentsToDiff(t *testing.T) {
	model := New(testDiff(), nil, nil)
	model = update(t, model, textKey("C"))
	model = update(t, model, textKey("q"))

	if model.mode != modeBrowse || model.quitting {
		t.Fatalf("mode=%v quitting=%v, want diff view without quitting", model.mode, model.quitting)
	}
}

func TestEmptyEditedCommentIsDeleted(t *testing.T) {
	t.Setenv("EDITOR", "true")
	comments := []review.StoredComment{{
		BatchID: "batch-1",
		Comment: review.Comment{Body: "old body"},
	}}
	var deleted review.StoredComment
	model := New(testDiff(), comments, nil)
	model.SetDelete(func(stored review.StoredComment, _ review.Diff) error {
		deleted = stored
		return nil
	})

	model = update(t, model, textKey("C"))
	model = update(t, model, specialKey(tea.KeyEnter))
	model = update(t, model, commentEditorFinishedMsg{body: "\n"})

	if deleted.BatchID != "batch-1" || len(model.comments) != 0 || model.mode != modeComments {
		t.Fatalf("deleted=%#v comments=%d mode=%v", deleted, len(model.comments), model.mode)
	}
}

func TestCommentsCanBeDeletedWithD(t *testing.T) {
	comments := []review.StoredComment{
		{BatchID: "batch-1", Index: 0, Comment: review.Comment{Body: "first"}},
		{BatchID: "batch-1", Index: 1, Comment: review.Comment{Body: "second"}},
	}
	var deleted review.StoredComment
	model := New(testDiff(), comments, nil)
	model.SetDelete(func(stored review.StoredComment, _ review.Diff) error {
		deleted = stored
		return nil
	})

	model = update(t, model, textKey("C"))
	model = update(t, model, textKey("D"))

	if deleted.Comment.Body != "first" || len(model.comments) != 1 {
		t.Fatalf("deleted=%#v comments=%#v", deleted, model.comments)
	}
	if model.comments[0].Index != 0 || !strings.Contains(model.renderComments(), "D delete") {
		t.Fatalf("remaining=%#v footer=%q", model.comments[0], model.renderComments())
	}
}

func TestCommentDeleteFailureKeepsCommentAndShowsError(t *testing.T) {
	comments := []review.StoredComment{{
		BatchID: "batch-1",
		Comment: review.Comment{Body: "keep me"},
	}}
	model := New(testDiff(), comments, nil)
	model.SetDelete(func(review.StoredComment, review.Diff) error {
		return fmt.Errorf("delete failed")
	})

	model = update(t, model, textKey("C"))
	model = update(t, model, textKey("D"))

	if len(model.comments) != 1 || !strings.Contains(ansi.Strip(model.renderComments()), "delete failed") {
		t.Fatalf("comments=%#v view=%q", model.comments, ansi.Strip(model.renderComments()))
	}
}

func TestSelectionCannotCrossHunk(t *testing.T) {
	model := New(testDiff(), nil, nil)
	model = update(t, model, textKey("v"))
	for range 10 {
		model = update(t, model, textKey("j"))
	}
	if model.rows[model.cursor].hunkIndex != 0 {
		t.Fatalf("selection crossed into hunk %d", model.rows[model.cursor].hunkIndex)
	}
}

func TestHalfPageMovementUsesRenderedRows(t *testing.T) {
	model := New(testDiff(), nil, nil)
	model.rows = make([]row, 30)
	for index := range model.rows {
		model.rows[index].kind = rowCode
	}
	model.rows[4].kind = rowFile
	model.rows[5].kind = rowHunk
	model.height = 11
	model.offset = 0
	model.cursor = 1
	delta := max(1, model.viewportHeight()/2)

	model = update(t, model, controlKey('d'))
	if model.offset != delta {
		t.Fatalf("Ctrl-d offset = %d, want %d rendered rows", model.offset, delta)
	}
	if model.cursor < model.offset || model.cursor >= model.offset+model.viewportHeight() {
		t.Fatalf("Ctrl-d cursor %d is outside viewport starting at %d", model.cursor, model.offset)
	}

	model = update(t, model, controlKey('u'))
	if model.offset != 0 {
		t.Fatalf("Ctrl-u offset = %d, want original viewport", model.offset)
	}
}

func TestVimSequencesAndLayoutToggle(t *testing.T) {
	model := New(testDiff(), nil, nil)
	model = update(t, model, tea.WindowSizeMsg{Width: 120, Height: 20})
	model = update(t, model, textKey("G"))
	if model.cursor != lastCodeRow(model.rows) {
		t.Fatalf("G cursor = %d, want %d", model.cursor, lastCodeRow(model.rows))
	}
	model = update(t, model, textKey("g"))
	model = update(t, model, textKey("g"))
	if model.cursor != firstCodeRow(model.rows) {
		t.Fatalf("gg cursor = %d, want %d", model.cursor, firstCodeRow(model.rows))
	}
	cursor := model.cursor
	model = update(t, model, textKey("]"))
	model = update(t, model, textKey("h"))
	if model.cursor != cursor {
		t.Fatalf("]h cursor = %d, want unchanged at %d", model.cursor, cursor)
	}
	model = update(t, model, textKey("t"))
	if !model.sideBySide {
		t.Fatal("side-by-side was not enabled")
	}
	if !strings.Contains(model.render(), "│") {
		t.Fatal("side-by-side render lacks divider")
	}
}

func TestZSequencesPositionCurrentLineInViewport(t *testing.T) {
	model := New(testDiff(), nil, nil)
	model.rows = make([]row, 20)
	model.cursor = 10
	model.height = 9

	for _, test := range []struct {
		key        string
		wantOffset int
	}{
		{key: "z", wantOffset: 7},
		{key: "t", wantOffset: 10},
		{key: "b", wantOffset: 5},
	} {
		model.offset = 0
		model = update(t, model, textKey("z"))
		model = update(t, model, textKey(test.key))
		if model.offset != test.wantOffset {
			t.Errorf("z%s offset = %d, want %d", test.key, model.offset, test.wantOffset)
		}
	}
}

func TestPendingKeyIsConsumedByNextKey(t *testing.T) {
	model := New(testDiff(), nil, nil)
	model = update(t, model, textKey("G"))
	last := model.cursor

	model = update(t, model, textKey("g"))
	model = update(t, model, textKey("h"))
	model = update(t, model, textKey("g"))

	if model.cursor != last {
		t.Fatalf("cursor = %d, want pending g consumed without jumping from %d", model.cursor, last)
	}
	if model.pendingKey != "g" {
		t.Fatalf("pending key = %q, want g", model.pendingKey)
	}
}

func TestStatusShowsBasicBindingsAndHelpShowsCompleteKeyMap(t *testing.T) {
	model := New(testDiff(), nil, nil)

	status := ansi.Strip(model.renderStatus())
	if !strings.HasPrefix(status, "j/k/h/l move  c comment  ? help  q quit") ||
		!strings.HasSuffix(status, "local changes") ||
		lipgloss.Width(status) != model.width {
		t.Fatalf("status = %q", status)
	}
	if strings.Contains(status, "select") || strings.Contains(status, "inbox") || strings.Contains(status, "layout") {
		t.Fatalf("status contains advanced bindings: %q", status)
	}

	help := ansi.Strip(model.renderHelp())
	if strings.Contains(help, "]h/[h") || strings.Contains(help, "next/previous hunk") {
		t.Fatalf("help retains hunk navigation binding:\n%s", help)
	}
	for _, binding := range []keyBinding{
		{keys: "v", description: "select a line range"},
		{keys: "e", description: "open current line in $EDITOR"},
		{keys: "C", description: "view comments"},
		{keys: "R", description: "refresh diff"},
		{keys: "/", description: "search diff text"},
		{keys: "n/N", description: "next/previous search match"},
		{keys: "zz/zt/zb", description: "center/top/bottom current line"},
		{keys: "Tab", description: "cycle local/parent branch changes"},
		{keys: "t", description: "toggle unified/side-by-side"},
	} {
		if !strings.Contains(help, binding.keys) || !strings.Contains(help, binding.description) {
			t.Fatalf("help does not contain %#v:\n%s", binding, help)
		}
	}
}

func TestFocusAndManualRefreshLoadCurrentView(t *testing.T) {
	model := New(testDiff(), nil, nil)
	model.SetParents([]string{"main"})
	model.target = 1
	var requested []string
	model.SetRefresh(func(parent string) (review.Diff, error) {
		requested = append(requested, parent)
		diff := testDiff()
		diff.Fingerprint = fmt.Sprintf("refresh-%d", len(requested))
		return diff, nil
	})

	next, cmd := model.Update(tea.FocusMsg{})
	model = next.(Model)
	if cmd == nil {
		t.Fatal("focus did not request a refresh")
	}
	model = update(t, model, cmd())

	next, cmd = model.Update(textKey("R"))
	model = next.(Model)
	if cmd == nil {
		t.Fatal("R did not request a refresh")
	}
	model = update(t, model, cmd())

	if len(requested) != 2 || requested[0] != "main" || requested[1] != "main" {
		t.Fatalf("refresh parents = %#v, want current branch twice", requested)
	}
	if model.diff.Fingerprint != "refresh-2" {
		t.Fatalf("fingerprint = %q, want second refresh", model.diff.Fingerprint)
	}
}

func TestHeaderShowsAddedAndRemovedLineCounts(t *testing.T) {
	model := New(testDiff(), nil, nil)
	header := strings.SplitN(ansi.Strip(model.render()), "\n", 2)[0]
	if header != "review-my-slop  +2-1" {
		t.Fatalf("header = %q", header)
	}
	if strings.Contains(header, "files") || strings.Contains(header, "pending") {
		t.Fatalf("header retains old summary: %q", header)
	}
}

func TestSearchMovesIncrementallyAndRepeats(t *testing.T) {
	model := New(testDiff(), nil, nil)
	start := model.cursor
	model = update(t, model, textKey("/"))
	model = update(t, model, textKey("keep"))

	if model.mode != modeSearch || model.rows[model.cursor].line.Text != "keep()" {
		t.Fatalf("search mode=%v cursor row=%#v", model.mode, model.rows[model.cursor])
	}
	first := model.cursor
	model = update(t, model, specialKey(tea.KeyEnter))
	model = update(t, model, textKey("n"))
	if model.cursor == first || model.rows[model.cursor].line.Text != "keep()" {
		t.Fatalf("next match cursor row=%#v", model.rows[model.cursor])
	}
	model = update(t, model, textKey("N"))
	if model.cursor != first {
		t.Fatalf("previous match cursor=%d, want %d", model.cursor, first)
	}

	model = update(t, model, textKey("/"))
	model = update(t, model, textKey("missing"))
	if !model.searchMiss || !strings.Contains(ansi.Strip(model.renderStatus()), "no matches") {
		t.Fatalf("missing search status=%q", ansi.Strip(model.renderStatus()))
	}
	model = update(t, model, specialKey(tea.KeyEsc))
	if model.cursor != first || model.mode != modeBrowse {
		t.Fatalf("cancel cursor=%d mode=%v, want cursor=%d browse", model.cursor, model.mode, first)
	}
	if start == first {
		t.Fatal("search did not move from its initial row")
	}
}

func TestSearchMatchesFileNamesAndBackspaceRestoresOrigin(t *testing.T) {
	model := New(testDiff(), nil, nil)
	origin := model.cursor
	model = update(t, model, textKey("/"))
	model = update(t, model, textKey("main.go"))
	if model.cursor != 0 {
		t.Fatalf("file search cursor=%d, want file header", model.cursor)
	}
	model = update(t, model, specialKey(tea.KeyBackspace))
	for range len("main.g") {
		model = update(t, model, specialKey(tea.KeyBackspace))
	}
	if model.cursor != origin || len(model.search) != 0 {
		t.Fatalf("cleared search cursor=%d query=%q, want origin=%d", model.cursor, model.search, origin)
	}
}

func TestTabCyclesParentBranchesAndIgnoresStaleRefresh(t *testing.T) {
	model := New(testDiff(), nil, nil)
	model.SetParents([]string{"stack-one", "main"})
	var requested string
	model.SetRefresh(func(parent string) (review.Diff, error) {
		requested = parent
		diff := testDiff()
		diff.Base = parent
		diff.Fingerprint = parent
		return diff, nil
	})

	next, cmd := model.Update(specialKey(tea.KeyTab))
	model = next.(Model)
	if model.currentParent() != "stack-one" || cmd == nil {
		t.Fatalf("parent = %q cmd = %v, want stack-one load", model.currentParent(), cmd)
	}
	msg := cmd()
	model = update(t, model, msg)
	if requested != "stack-one" || model.diff.Fingerprint != "stack-one" {
		t.Fatalf("requested parent=%q fingerprint=%q", requested, model.diff.Fingerprint)
	}
	if rendered := ansi.Strip(model.renderStatus()); !strings.HasSuffix(rendered, "branch changes from stack-one") {
		t.Fatalf("branch view lacks mode label:\n%s", rendered)
	}

	stale := testDiff()
	stale.Fingerprint = "stale-local"
	model = update(t, model, refreshDiffMsg{diff: stale})
	if model.diff.Fingerprint != "stack-one" {
		t.Fatalf("stale local refresh replaced branch diff: %q", model.diff.Fingerprint)
	}

	next, cmd = model.Update(specialKey(tea.KeyTab))
	model = next.(Model)
	model = update(t, model, cmd())
	if model.currentParent() != "main" || requested != "main" {
		t.Fatalf("second parent=%q requested=%q", model.currentParent(), requested)
	}
	next, cmd = model.Update(specialKey(tea.KeyTab))
	model = next.(Model)
	model = update(t, model, cmd())
	if model.currentParent() != "" || requested != "" {
		t.Fatalf("wrapped parent=%q requested=%q, want local", model.currentParent(), requested)
	}
}

func TestRenderKeyBindingsAlignsDescriptions(t *testing.T) {
	lines := renderKeyBindings([]keyBinding{
		{keys: "x", description: "alpha"},
		{keys: "long keys", description: "beta"},
	})
	if strings.Index(lines[0], "alpha") != strings.Index(lines[1], "beta") {
		t.Fatalf("descriptions are not aligned: %#v", lines)
	}
}

func TestDiffRefreshPreservesCursorWhenRowsShift(t *testing.T) {
	model := New(testDiff(), nil, nil)
	model.cursor = findCodeRow(t, model, review.LineAdded)
	current := model.rows[model.cursor].line

	refreshed := testDiff()
	refreshed.Fingerprint = "refreshed"
	refreshed.Files[0].Metadata = []string{"new metadata row"}
	model = update(t, model, refreshDiffMsg{diff: refreshed})

	if model.diff.Fingerprint != "refreshed" {
		t.Fatalf("fingerprint = %q, want refreshed", model.diff.Fingerprint)
	}
	if got := model.rows[model.cursor].line; got != current {
		t.Fatalf("cursor moved to %#v, want %#v", got, current)
	}
}

func TestCommentAfterRefreshUsesCurrentDiff(t *testing.T) {
	t.Setenv("EDITOR", "true")
	var savedWith review.Diff
	model := New(testDiff(), nil, func(stored review.StoredComment, diff review.Diff) (review.StoredComment, error) {
		savedWith = diff
		stored.BatchID = "new"
		return stored, nil
	})
	refreshed := testDiff()
	refreshed.Fingerprint = "refreshed"
	model = update(t, model, refreshDiffMsg{diff: refreshed})
	model = update(t, model, textKey("c"))
	_ = update(t, model, commentEditorFinishedMsg{body: "comment"})

	if savedWith.Fingerprint != "refreshed" {
		t.Fatalf("saved with fingerprint %q, want refreshed", savedWith.Fingerprint)
	}
}

func TestSideBySideTabsDoNotShiftLineNumbers(t *testing.T) {
	model := New(testDiff(), nil, nil)
	model.width = 120
	model.sideBySide = true
	index := findCodeRow(t, model, review.LineContext)
	model.rows[index].text = "\t\tif err != nil { return fmt.Errorf(\"a deliberately long line\") }"

	rendered := ansi.Strip(model.renderRow(index))
	if strings.ContainsRune(rendered, '\t') {
		t.Fatalf("rendered row contains a tab: %q", rendered)
	}
	if divider := strings.Index(rendered, "│"); divider != 59 {
		t.Fatalf("divider column = %d, want 59: %q", divider, rendered)
	}
	if width := lipgloss.Width(rendered); width != 120 {
		t.Fatalf("row width = %d, want 120", width)
	}
}

func TestHorizontalScrollKeepsUnifiedGutterFixed(t *testing.T) {
	model := New(testDiff(), nil, nil)
	model.width = 37
	index := findCodeRow(t, model, review.LineContext)
	model.cursor = index
	model.rows[index].text = "abcdefghijklmnopqrstuvwxyz0123456789"

	before := ansi.Strip(model.renderRow(index))
	model = update(t, model, textKey("l"))
	model = update(t, model, textKey("l"))
	after := ansi.Strip(model.renderRow(index))

	if before[:14] != after[:14] {
		t.Fatalf("gutter moved: before=%q after=%q", before[:14], after[:14])
	}
	if !strings.Contains(after[14:], "cdefgh") {
		t.Fatalf("scrolled content = %q, want content starting at offset 2", after[14:])
	}
	if model.xOffset != 2 {
		t.Fatalf("horizontal offset = %d, want 2", model.xOffset)
	}
}

func TestHorizontalScrollKeepsSideBySideGuttersAndDividerFixed(t *testing.T) {
	model := New(testDiff(), nil, nil)
	model.width = 120
	model.sideBySide = true
	index := findCodeRow(t, model, review.LineContext)
	model.rows[index].text = "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	model.xOffset = 8

	rendered := ansi.Strip(model.renderRow(index))
	if divider := strings.Index(rendered, "│"); divider != 59 {
		t.Fatalf("divider column = %d, want 59: %q", divider, rendered)
	}
	if rendered[:6] != "    1 " || rendered[63:69] != "    1 " {
		t.Fatalf("line-number gutters moved: %q", rendered)
	}
	if !strings.Contains(rendered[6:59], "ghijkl") ||
		!strings.Contains(rendered[69:], "ghijkl") {
		t.Fatalf("panes did not share horizontal offset: %q", rendered)
	}
}

func TestHorizontalScrollStartAndEnd(t *testing.T) {
	model := New(testDiff(), nil, nil)
	model.width = 37
	index := findCodeRow(t, model, review.LineContext)
	model.rows[index].text = strings.Repeat("x", 60)

	model = update(t, model, textKey("$"))
	if model.xOffset != 37 {
		t.Fatalf("end offset = %d, want 37", model.xOffset)
	}
	model = update(t, model, textKey("0"))
	if model.xOffset != 0 {
		t.Fatalf("start offset = %d, want 0", model.xOffset)
	}
}

func TestDiffMarkersUseTerminalColorsAndCursorFillsTerminalWidth(t *testing.T) {
	model := New(testDiff(), nil, nil)
	model = update(t, model, tea.WindowSizeMsg{Width: 80, Height: 20})

	addedIndex := findCodeRow(t, model, review.LineAdded)
	added := model.renderRow(addedIndex)
	if !strings.Contains(added, "\x1b[32m+\x1b[m") {
		t.Fatalf("added row does not use terminal green: %q", added)
	}
	if !strings.Contains(added, "[38;2;") || !strings.Contains(added, "new") {
		t.Fatalf("added row lost syntax highlighting: %q", added)
	}

	removedIndex := findCodeRow(t, model, review.LineRemoved)
	removed := model.renderRow(removedIndex)
	if !strings.Contains(removed, "\x1b[31m-\x1b[m") {
		t.Fatalf("removed row does not use terminal red: %q", removed)
	}
	if !strings.Contains(removed, "[38;2;") || !strings.Contains(removed, "old") {
		t.Fatalf("removed row lost syntax highlighting: %q", removed)
	}

	model.cursor = addedIndex
	cursor := model.renderRow(addedIndex)
	assertStyledThroughColumn(t, cursor, 80, sgrExpectation{reverse: true})

	contextIndex := findCodeRow(t, model, review.LineContext)
	model.cursor = contextIndex
	contextCursor := model.renderRow(contextIndex)
	assertStyledThroughColumn(t, contextCursor, 80, sgrExpectation{reverse: true})
}

func TestSelectionBackgroundKeepsDefaultCursorAndTextWeight(t *testing.T) {
	model := New(testDiff(), nil, nil)
	model = update(t, model, tea.WindowSizeMsg{Width: 72, Height: 20})
	model.dark = false
	model.selectFrom = findCodeRow(t, model, review.LineRemoved)
	model.cursor = findCodeRow(t, model, review.LineAdded)
	model.selecting = true

	selected := model.renderRow(model.selectFrom)
	cursor := model.renderRow(model.cursor)
	assertStyledThroughColumn(t, selected, 72, sgrExpectation{background: "219;234;254"})
	assertStyledThroughColumn(t, cursor, 72, sgrExpectation{reverse: true})
	for name, rendered := range map[string]string{"selected": selected, "cursor": cursor} {
		if strings.Contains(rendered, "\x1b[1m") {
			t.Fatalf("%s row uses bold text: %q", name, rendered)
		}
	}
}

func TestRenderedCodeRowsHaveExactTerminalWidth(t *testing.T) {
	model := New(testDiff(), nil, nil)
	model = update(t, model, tea.WindowSizeMsg{Width: 37, Height: 20})
	for index, row := range model.rows {
		if row.kind != rowCode {
			continue
		}
		if width := lipgloss.Width(model.renderRow(index)); width != 37 {
			t.Fatalf("row %d width = %d, want 37", index, width)
		}
	}
}

func TestRenderStyledRowStripsSyntaxBackgroundColors(t *testing.T) {
	value := strings.Join([]string{
		"\x1b[48;2;255;0;0;38;2;1;2;3mtruecolor",
		"\x1b[48;5;123;1mindexed",
		"\x1b[45mstandard",
		"\x1b[105mbright",
	}, " ")
	rendered := renderStyledRow(addedStyle, value, 80, false)
	for _, forbidden := range []string{"48;2;255;0;0", "48;5;123", "[45m", "[105m"} {
		if strings.Contains(rendered, forbidden) {
			t.Fatalf("rendered row retains background sequence %q: %q", forbidden, rendered)
		}
	}
}

func TestViewPreservesTerminalColors(t *testing.T) {
	view := New(testDiff(), nil, nil).View()
	if view.BackgroundColor != nil {
		t.Fatalf("background override = %#v, want nil", view.BackgroundColor)
	}
	if view.ForegroundColor != nil {
		t.Fatalf("foreground override = %#v, want nil", view.ForegroundColor)
	}
	if !view.AltScreen {
		t.Fatal("view does not use the alternate screen")
	}
	if !view.ReportFocus {
		t.Fatal("view does not request terminal focus events")
	}
}

func update(t *testing.T, model Model, msg tea.Msg) Model {
	t.Helper()
	next, _ := model.Update(msg)
	got, ok := next.(Model)
	if !ok {
		t.Fatalf("model type = %T", next)
	}
	return got
}

func textKey(text string) tea.KeyPressMsg {
	runes := []rune(text)
	return tea.KeyPressMsg(tea.Key{Text: text, Code: runes[0]})
}

func specialKey(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Code: code})
}

func controlKey(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Code: code, Mod: tea.ModCtrl})
}

func findCodeRow(t *testing.T, model Model, kind review.LineKind) int {
	t.Helper()
	for index, row := range model.rows {
		if row.kind == rowCode && row.line.Kind == kind {
			return index
		}
	}
	t.Fatalf("no code row with kind %v", kind)
	return -1
}

type sgrExpectation struct {
	background string
	foreground string
	reverse    bool
}

var sgrPattern = regexp.MustCompile(`\x1b\[([0-9;]*)m`)

func assertStyledThroughColumn(t *testing.T, rendered string, width int, expected sgrExpectation) {
	t.Helper()
	state := sgrState{}
	column := 0
	for len(rendered) > 0 {
		location := sgrPattern.FindStringSubmatchIndex(rendered)
		if location == nil {
			for range rendered {
				column++
				assertSGRState(t, column, state, expected)
			}
			break
		}
		for range rendered[:location[0]] {
			column++
			assertSGRState(t, column, state, expected)
		}
		state.apply(rendered[location[2]:location[3]])
		rendered = rendered[location[1]:]
	}
	if column != width {
		t.Fatalf("styled columns = %d, want %d", column, width)
	}
}

type sgrState struct {
	background string
	foreground string
	reverse    bool
}

func (s *sgrState) apply(parameters string) {
	if parameters == "" {
		parameters = "0"
	}
	values := strings.Split(parameters, ";")
	for index := 0; index < len(values); index++ {
		value, _ := strconv.Atoi(values[index])
		switch value {
		case 0:
			*s = sgrState{}
		case 7:
			s.reverse = true
		case 27:
			s.reverse = false
		case 39:
			s.foreground = ""
		case 49:
			s.background = ""
		case 38:
			if index+4 < len(values) && values[index+1] == "2" {
				s.foreground = strings.Join(values[index+2:index+5], ";")
				index += 4
			}
		case 48:
			if index+4 < len(values) && values[index+1] == "2" {
				s.background = strings.Join(values[index+2:index+5], ";")
				index += 4
			}
		}
	}
}

func assertSGRState(t *testing.T, column int, state sgrState, expected sgrExpectation) {
	t.Helper()
	if state.background != expected.background {
		t.Fatalf("column %d background = %q, want %q", column, state.background, expected.background)
	}
	if expected.foreground != "" && state.foreground != expected.foreground {
		t.Fatalf("column %d foreground = %q, want %q", column, state.foreground, expected.foreground)
	}
	if state.reverse != expected.reverse {
		t.Fatalf("column %d reverse = %v, want %v", column, state.reverse, expected.reverse)
	}
}

func testDiff() review.Diff {
	return review.Diff{
		Repository:  "/repo",
		Fingerprint: "fingerprint",
		Files: []review.File{{
			Display:   "main.go",
			OldPath:   "main.go",
			NewPath:   "main.go",
			Language:  "main.go",
			OldSource: "package main\nold()\nkeep()\n",
			NewSource: "package main\nnew()\nkeep()\nmore()\n",
			Hunks: []review.Hunk{
				{
					Header: "@@ -1,3 +1,3 @@",
					Lines: []review.Line{
						{Kind: review.LineContext, Text: "package main", OldNumber: 1, NewNumber: 1},
						{Kind: review.LineRemoved, Text: "old()", OldNumber: 2},
						{Kind: review.LineAdded, Text: "new()", NewNumber: 2},
						{Kind: review.LineContext, Text: "keep()", OldNumber: 3, NewNumber: 3},
					},
				},
				{
					Header: "@@ -3,1 +3,2 @@",
					Lines: []review.Line{
						{Kind: review.LineContext, Text: "keep()", OldNumber: 3, NewNumber: 3},
						{Kind: review.LineAdded, Text: "more()", NewNumber: 4},
					},
				},
			},
		}},
	}
}
