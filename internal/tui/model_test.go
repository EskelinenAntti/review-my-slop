package tui

import (
	"fmt"
	"os"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/anttieskelinen/review-my-slop/internal/review"
)

func TestVisualSelectionCreatesMappedAnchorAndSubmits(t *testing.T) {
	var saved []review.Comment
	model := New(testDiff(), nil, func(stored review.StoredComment) (review.StoredComment, error) {
		saved = append(saved, stored.Comment)
		stored.BatchID = "new"
		return stored, nil
	})

	model = update(t, model, textKey("j"))
	model = update(t, model, textKey("v"))
	model = update(t, model, textKey("j"))
	model = update(t, model, textKey("c"))
	if model.mode != modeComment {
		t.Fatalf("mode = %v, want comment", model.mode)
	}
	for _, r := range "fix both lines" {
		model = update(t, model, textKey(string(r)))
	}
	model = update(t, model, specialKey(tea.KeyEnter))
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

func TestCommentEditorShortcuts(t *testing.T) {
	var saved review.Comment
	model := New(testDiff(), nil, func(stored review.StoredComment) (review.StoredComment, error) {
		saved = stored.Comment
		stored.BatchID = "new"
		return stored, nil
	})
	model = update(t, model, textKey("c"))
	model = update(t, model, textKey("first"))
	model = update(t, model, modifiedKey(tea.KeyEnter, tea.ModShift))
	model = update(t, model, textKey("second"))

	if got := string(model.editor); got != "first\nsecond" {
		t.Fatalf("editor = %q, want multiline comment", got)
	}
	model = update(t, model, specialKey(tea.KeyEnter))
	if saved.Body != "first\nsecond" {
		t.Fatalf("comment = %#v, want multiline comment", saved)
	}
}

func TestCommentSaveFailureKeepsDraftOpen(t *testing.T) {
	model := New(testDiff(), nil, func(review.StoredComment) (review.StoredComment, error) {
		return review.StoredComment{}, fmt.Errorf("inbox unavailable")
	})
	model = update(t, model, textKey("c"))
	model = update(t, model, textKey("keep this"))
	model = update(t, model, specialKey(tea.KeyEnter))

	if model.mode != modeComment || string(model.editor) != "keep this" {
		t.Fatalf("failed save lost draft: mode=%v editor=%q", model.mode, model.editor)
	}
	if model.err == nil || model.err.Error() != "inbox unavailable" {
		t.Fatalf("error = %v, want inbox failure", model.err)
	}
}

func TestExternalEditorResultReplacesDraft(t *testing.T) {
	model := New(testDiff(), nil, nil)
	model = update(t, model, textKey("c"))
	model.editor = []rune("old draft")

	model = update(t, model, externalEditorFinishedMsg{body: "edited externally\n"})
	if got := string(model.editor); got != "edited externally\n" {
		t.Fatalf("editor = %q, want external editor contents", got)
	}
	if model.mode != modeComment {
		t.Fatalf("mode = %v, want comment", model.mode)
	}
}

func TestCtrlGRequiresEditor(t *testing.T) {
	t.Setenv("EDITOR", "")
	model := New(testDiff(), nil, nil)
	model = update(t, model, textKey("c"))
	model = update(t, model, controlKey('g'))

	if model.err == nil || model.err.Error() != "$EDITOR is not set" {
		t.Fatalf("error = %v, want missing editor error", model.err)
	}
}

func TestExternalEditorCommandReadsEditedDraft(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test editor command uses sh")
	}
	file, err := os.CreateTemp("", "review-my-slop-editor-test-*.md")
	if err != nil {
		t.Fatal(err)
	}
	path := file.Name()
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(path) })

	if err := editorCommand("printf 'edited externally' >", path).Run(); err != nil {
		t.Fatal(err)
	}
	msg := readExternalEditorResult(path, nil)
	if msg.err != nil {
		t.Fatal(msg.err)
	}
	if msg.body != "edited externally" {
		t.Fatalf("body = %q, want external editor contents", msg.body)
	}
}

func TestInboxCommentsCanBeViewedAndEdited(t *testing.T) {
	comments := []review.StoredComment{{
		BatchID: "batch-1",
		Comment: review.Comment{
			Anchor: review.Anchor{File: "main.go", NewStart: 2},
			Body:   "old body",
		},
	}}
	var persisted review.StoredComment
	model := New(testDiff(), comments, func(stored review.StoredComment) (review.StoredComment, error) {
		persisted = stored
		return stored, nil
	})

	model = update(t, model, textKey("C"))
	if model.mode != modeComments || !strings.Contains(model.render(), "old body") {
		t.Fatal("inbox comments view did not open")
	}
	model = update(t, model, specialKey(tea.KeyEnter))
	model.editor = []rune("edited body")
	model = update(t, model, specialKey(tea.KeyEnter))

	if persisted.BatchID != "batch-1" || persisted.Comment.Body != "edited body" {
		t.Fatalf("persisted = %#v, want edited existing comment", persisted)
	}
	if model.mode != modeComments || model.comments[0].Comment.Body != "edited body" {
		t.Fatal("edited comment was not reflected in inbox view")
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
	model = update(t, model, textKey("]"))
	model = update(t, model, textKey("h"))
	if model.rows[model.cursor].hunkIndex != 1 {
		t.Fatalf("]h hunk = %d, want 1", model.rows[model.cursor].hunkIndex)
	}
	model = update(t, model, textKey("t"))
	if !model.sideBySide {
		t.Fatal("side-by-side was not enabled")
	}
	if !strings.Contains(model.render(), "│") {
		t.Fatal("side-by-side render lacks divider")
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

func TestDiffBackgroundAndCursorFillTerminalWidth(t *testing.T) {
	model := New(testDiff(), nil, nil)
	model = update(t, model, tea.WindowSizeMsg{Width: 80, Height: 20})

	addedIndex := findCodeRow(t, model, review.LineAdded)
	added := model.renderRow(addedIndex)
	assertStyledThroughColumn(t, added, 80, sgrExpectation{background: "52;67;47"})

	removedIndex := findCodeRow(t, model, review.LineRemoved)
	removed := model.renderRow(removedIndex)
	assertStyledThroughColumn(t, removed, 80, sgrExpectation{background: "75;48;46"})

	model.cursor = addedIndex
	cursor := model.renderRow(addedIndex)
	assertStyledThroughColumn(t, cursor, 80, sgrExpectation{
		background: "250;189;47",
		foreground: "40;40;40",
	})

	contextIndex := findCodeRow(t, model, review.LineContext)
	model.cursor = contextIndex
	contextCursor := model.renderRow(contextIndex)
	assertStyledThroughColumn(t, contextCursor, 80, sgrExpectation{
		background: "250;189;47",
		foreground: "40;40;40",
	})
}

func TestSelectionBackgroundSurvivesSyntaxHighlightResets(t *testing.T) {
	model := New(testDiff(), nil, nil)
	model = update(t, model, tea.WindowSizeMsg{Width: 72, Height: 20})
	model.cursor = findCodeRow(t, model, review.LineAdded)
	model.selecting = true
	model.selectFrom = model.cursor

	rendered := model.renderRow(model.cursor)
	assertStyledThroughColumn(t, rendered, 72, sgrExpectation{
		background: "250;189;47",
		foreground: "40;40;40",
	})
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
	assertStyledThroughColumn(t, rendered, 80, sgrExpectation{background: "52;67;47"})
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

func controlKey(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Code: code, Mod: tea.ModCtrl})
}

func specialKey(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Code: code})
}

func modifiedKey(code rune, mod tea.KeyMod) tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Code: code, Mod: mod})
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
