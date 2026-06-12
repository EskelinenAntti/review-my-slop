package tui

import (
	"regexp"
	"strconv"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/anttieskelinen/review-my-slop/internal/review"
)

func TestVisualSelectionCreatesMappedAnchorAndSubmits(t *testing.T) {
	var submitted []review.Comment
	model := New(testDiff(), func(comments []review.Comment) error {
		submitted = comments
		return nil
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
	model = update(t, model, controlKey('s'))
	if len(model.comments) != 1 {
		t.Fatalf("comments = %d, want 1", len(model.comments))
	}
	anchor := model.comments[0].Anchor
	if anchor.File != "main.go" || anchor.OldStart != 2 || anchor.OldEnd != 2 ||
		anchor.NewStart != 2 || anchor.NewEnd != 2 {
		t.Fatalf("unexpected anchor: %#v", anchor)
	}
	if len(anchor.QuotedLines) != 2 ||
		anchor.QuotedLines[0] != "-old()" ||
		anchor.QuotedLines[1] != "+new()" {
		t.Fatalf("unexpected quoted lines: %#v", anchor.QuotedLines)
	}

	model = update(t, model, textKey("s"))
	if len(submitted) != 1 || submitted[0].Body != "fix both lines" {
		t.Fatalf("unexpected submitted comments: %#v", submitted)
	}
}

func TestSelectionCannotCrossHunk(t *testing.T) {
	model := New(testDiff(), nil)
	model = update(t, model, textKey("v"))
	for range 10 {
		model = update(t, model, textKey("j"))
	}
	if model.rows[model.cursor].hunkIndex != 0 {
		t.Fatalf("selection crossed into hunk %d", model.rows[model.cursor].hunkIndex)
	}
}

func TestVimSequencesAndLayoutToggle(t *testing.T) {
	model := New(testDiff(), nil)
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

func TestQuitRequiresConfirmationWithComments(t *testing.T) {
	model := New(testDiff(), nil)
	model.comments = []review.Comment{{Body: "pending"}}
	model = update(t, model, textKey("q"))
	if model.mode != modeConfirmQuit {
		t.Fatalf("mode = %v, want quit confirmation", model.mode)
	}
	model = update(t, model, textKey("n"))
	if model.mode != modeBrowse || model.quitting {
		t.Fatal("quit cancellation failed")
	}
}

func TestDiffBackgroundAndCursorFillTerminalWidth(t *testing.T) {
	model := New(testDiff(), nil)
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
	model := New(testDiff(), nil)
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
	model := New(testDiff(), nil)
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
	view := New(testDiff(), nil).View()
	if view.BackgroundColor != nil {
		t.Fatalf("background override = %#v, want nil", view.BackgroundColor)
	}
	if view.ForegroundColor != nil {
		t.Fatalf("foreground override = %#v, want nil", view.ForegroundColor)
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
