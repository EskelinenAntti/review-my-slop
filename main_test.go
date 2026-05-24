package slop

import (
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/anttieskelinen/review-my-slop/internal/ansi"
	"github.com/anttieskelinen/review-my-slop/internal/github"
)

func TestParseDiff(t *testing.T) {
	diff := []byte(`diff --git a/a.go b/a.go
index 1111111..2222222 100644
--- a/a.go
+++ b/a.go
@@ -2 +2,2 @@
-old line
+new line
+another line
`)

	refs := parseDiff(diff)
	if len(refs) != 3 {
		t.Fatalf("expected 3 refs, got %d", len(refs))
	}

	tests := []struct {
		index   int
		file    string
		line    int
		side    string
		content string
	}{
		{1, "a.go", 2, "old", "old line"},
		{2, "a.go", 2, "new", "new line"},
		{3, "a.go", 3, "new", "another line"},
	}

	for i, want := range tests {
		got := refs[i]
		if got.Index != want.index || got.File != want.file || got.Line != want.line || got.Side != want.side || got.Content != want.content {
			t.Fatalf("ref %d = %#v, want %#v", i, got, want)
		}
	}
}

func TestChangedLinesIncludesUntrackedFile(t *testing.T) {
	withTempGitRepo(t)
	if err := os.WriteFile("new.txt", []byte("first\nsecond\n"), 0644); err != nil {
		t.Fatal(err)
	}

	refs, err := changedLineRefs(nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 2 {
		t.Fatalf("refs = %#v, want two untracked lines", refs)
	}
	if refs[0].File != "new.txt" || refs[0].Line != 1 || refs[0].Side != "new" || refs[0].Content != "first" {
		t.Fatalf("first untracked ref = %#v", refs[0])
	}
	if refs[1].File != "new.txt" || refs[1].Line != 2 || refs[1].Side != "new" || refs[1].Content != "second" {
		t.Fatalf("second untracked ref = %#v", refs[1])
	}
}

func TestDiffWithUntrackedFilesShowsPath(t *testing.T) {
	withTempGitRepo(t)
	if err := os.WriteFile("new.txt", []byte("first\n"), 0644); err != nil {
		t.Fatal(err)
	}

	got := string(diffWithUntrackedFiles(nil, nil))
	if !strings.Contains(got, "new.txt --- Text") || !strings.Contains(got, "first") {
		t.Fatalf("untracked diff = %q, want path header and file content", got)
	}
}

func withTempGitRepo(t *testing.T) {
	t.Helper()

	dir := t.TempDir()
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(previous); err != nil {
			t.Fatal(err)
		}
	})
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "init")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %s", strings.TrimSpace(string(out)))
	}
}

func TestTruncateANSIPreservesEscapeSequences(t *testing.T) {
	got := ansi.Truncate("\x1b[31mabcdef\x1b[0m", 4)
	if !strings.Contains(got, "\x1b[31m") {
		t.Fatalf("expected ansi escapes in %q", got)
	}
	if ansi.VisibleLen(got) != 4 {
		t.Fatalf("visible length = %d, want 4", ansi.VisibleLen(got))
	}
}

func TestBuildChangedLinesUsesDisplayedRows(t *testing.T) {
	lines := []string{
		"\x1b[1mmain.go\x1b[0m --- Go",
		"\x1b[2m 10 \x1b[0m old text        \x1b[2m 10 \x1b[0m new text",
		"\x1b[2m .. \x1b[0m wrapped text     \x1b[2m .. \x1b[0m wrapped text",
		"\x1b[2m .. \x1b[0m                  \x1b[92;1m 12 \x1b[0m added only",
		"\x1b[91;1m 11 \x1b[0m removed       \x1b[92;1m 11 \x1b[0m added",
	}

	changedLines := buildChangedLines(lines, nil)
	if len(changedLines) != 2 {
		t.Fatalf("expected 2 selectable rows, got %d", len(changedLines))
	}
	if changedLines[0].LineIndex != 3 || changedLines[0].Ref.Line != 12 || changedLines[0].Ref.Side != "new" {
		t.Fatalf("first changed line = %#v", changedLines[0])
	}
	if changedLines[1].LineIndex != 4 || changedLines[1].Ref.Line != 11 || changedLines[1].Ref.Side != "new" {
		t.Fatalf("second changed line = %#v", changedLines[1])
	}
	if changedLines[1].Left == nil || changedLines[1].Left.Line != 11 || changedLines[1].Left.Side != "old" {
		t.Fatalf("second changed line left side = %#v", changedLines[1].Left)
	}
	if changedLines[1].Right == nil || changedLines[1].Right.Line != 11 || changedLines[1].Right.Side != "new" {
		t.Fatalf("second changed line right side = %#v", changedLines[1].Right)
	}
}

func TestBuildChangedLinesDoesNotTreatZeroInContentAsLineNumber(t *testing.T) {
	lines := []string{
		"\x1b[1minternal/github/github.go\x1b[0m --- Go",
		"\x1b[92;1m 283 \x1b[0mif len(response.Data) == 0 {",
	}

	changedLines := buildChangedLines(lines, nil)
	if len(changedLines) != 1 {
		t.Fatalf("expected one selectable row, got %d", len(changedLines))
	}
	if changedLines[0].Ref.Side != "new" || changedLines[0].Ref.Line != 283 {
		t.Fatalf("changed line = %#v, want new side line 283", changedLines[0])
	}
	if changedLines[0].Right == nil || changedLines[0].Right.Line != 283 {
		t.Fatalf("right side = %#v, want line 283", changedLines[0].Right)
	}
	if changedLines[0].Left != nil {
		t.Fatalf("left side = %#v, want nil for single-sided added row", changedLines[0].Left)
	}
}

func TestBuildChangedLinesKeepsAddedLineContainingTripleDashSelectable(t *testing.T) {
	lines := []string{
		"\x1b[1mmain.go\x1b[0m --- Go",
		"\x1b[92;1m 561 \x1b[0mbuf.WriteString(\" --- Text\\n\")",
	}

	changedLines := buildChangedLines(lines, nil)
	if len(changedLines) != 1 {
		t.Fatalf("expected one selectable row, got %d", len(changedLines))
	}
	if changedLines[0].LineIndex != 1 || changedLines[0].Ref.Line != 561 || changedLines[0].Ref.Side != "new" {
		t.Fatalf("changed line = %#v, want new side line 561", changedLines[0])
	}
}

func TestBuildChangedLinesSplitsBeforeRightLineNumber(t *testing.T) {
	lines := []string{
		"\x1b[1mmain.go\x1b[0m --- Go",
		"\x1b[91m 1387 old\x1b[0m       \x1b[92m 537 new\x1b[0m",
	}

	changedLines := buildChangedLines(lines, nil)
	if len(changedLines) != 1 {
		t.Fatalf("expected one selectable row, got %d", len(changedLines))
	}
	rightLineStart := strings.Index(ansi.Strip(lines[1]), "537")
	if rightLineStart < 0 {
		t.Fatalf("right line number not found in %q", ansi.Strip(lines[1]))
	}
	if changedLines[0].Split > rightLineStart {
		t.Fatalf("split = %d, want before or at right line number column %d", changedLines[0].Split, rightLineStart)
	}
}

func TestHighlightPlainStripsDiffColors(t *testing.T) {
	got := highlightPlain("\x1b[31mred\x1b[0m plain", 12)
	if strings.Contains(got, "\x1b[31m") {
		t.Fatalf("expected diff color to be stripped from %q", got)
	}
	if ansi.VisibleLen(got) != 12 {
		t.Fatalf("visible length = %d, want 12", ansi.VisibleLen(got))
	}
}

func TestHighlightChangedLineSidePreservesColorsOutsideCursor(t *testing.T) {
	row := changedLine{
		Ref:   lineRef{Side: "new"},
		Split: 8,
	}
	got := highlightChangedLineSide("\x1b[31mremoved\x1b[0m  \x1b[32madded\x1b[0m", 16, row)
	if !strings.Contains(got, "\x1b[31m") {
		t.Fatalf("expected left-side color to be preserved in %q", got)
	}
	if strings.Contains(got, "\x1b[32m") {
		t.Fatalf("expected selected right-side color to be suppressed in %q", got)
	}
	if !strings.Contains(got, "\x1b[7m") {
		t.Fatalf("expected inverse range in %q", got)
	}
}

func TestHighlightANSIRangeSuppressesStylesInsideCursor(t *testing.T) {
	got := highlightANSIRange("ab\x1b[0mcd", 6, 0, 6)
	if strings.Count(got, "\x1b[7m") != 1 {
		t.Fatalf("expected one inverse span in %q", got)
	}
	if strings.Contains(got, "ab\x1b[0mcd") {
		t.Fatalf("expected reset inside cursor to be suppressed in %q", got)
	}
	if ansi.VisibleLen(got) != 6 {
		t.Fatalf("visible length = %d, want 6", ansi.VisibleLen(got))
	}
}

func TestDisplayLineSelectionIncludesIntermediateRows(t *testing.T) {
	anchor := 0
	state := &reviewState{
		cursor:          1,
		selectionAnchor: &anchor,
		lines: []string{
			"\x1b[2m 10 \x1b[0m old        \x1b[2m 10 \x1b[0m new",
			"unchanged context between changed rows",
			"\x1b[2m 12 \x1b[0m old        \x1b[2m 12 \x1b[0m new",
		},
		changedLines: []changedLine{
			{LineIndex: 0, Ref: lineRef{File: "a.go", Line: 10, Side: "new"}, Split: 20},
			{LineIndex: 2, Ref: lineRef{File: "a.go", Line: 12, Side: "new"}, Split: 20},
		},
	}

	changedLine, ok := state.displayLineSelection(1, 80)
	if !ok {
		t.Fatal("expected intermediate display line to be highlighted")
	}
	if changedLine.Ref.Side != "new" {
		t.Fatalf("intermediate changed line side = %q, want new", changedLine.Ref.Side)
	}
}

func TestRenderCursorKeepsInverseAcrossLineColorResets(t *testing.T) {
	state := &reviewState{
		lines: []string{
			"\x1b[92m 10 right side\x1b[0m",
		},
		changedLines: []changedLine{
			testChangedLine(lineRef{File: "a.go", Line: 10, Side: "new", Content: "right side"}),
		},
	}
	var out strings.Builder
	render(&out, state, 8, 40)

	got := out.String()
	if !strings.Contains(got, "\x1b[92m") {
		t.Fatalf("expected render to preserve line color in %q", got)
	}
	highlightStart := strings.Index(got, "\x1b[7m")
	if highlightStart < 0 {
		t.Fatalf("expected cursor inverse in %q", got)
	}
	highlightEnd := strings.Index(got[highlightStart:], "\x1b[0m")
	if highlightEnd < 0 {
		t.Fatalf("expected cursor reset in %q", got)
	}
	highlighted := got[highlightStart : highlightStart+highlightEnd]
	if strings.Contains(highlighted, "\x1b[92m") {
		t.Fatalf("expected selected cursor span to suppress line color in %q", got)
	}
}

func TestSelectionMovementStaysWithinFileAndSide(t *testing.T) {
	state := &reviewState{
		source: sourceBranch,
		pr:     &prContext{Head: "head", Base: "base"},
		draft:  reviewDraft{Active: true},
		changedLines: []changedLine{
			testChangedLine(lineRef{File: "a.go", Line: 10, Side: "new"}),
			testChangedLine(lineRef{File: "a.go", Line: 11, Side: "new"}),
			testChangedLine(lineRef{File: "a.go", Line: 12, Side: "old"}),
			testChangedLine(lineRef{File: "b.go", Line: 1, Side: "new"}),
		},
	}
	state.toggleSelection()
	state.move(1)
	if state.cursor != 1 {
		t.Fatalf("cursor after valid move = %d, want 1", state.cursor)
	}
	state.move(1)
	if state.cursor != 1 {
		t.Fatalf("cursor after side-violating move = %d, want 1", state.cursor)
	}
	state.moveTo(3)
	if state.cursor != 1 {
		t.Fatalf("cursor after file-violating move = %d, want 1", state.cursor)
	}
}

func TestCurrentRangeSortsByLine(t *testing.T) {
	anchor := 1
	state := &reviewState{
		cursor:          0,
		selectionAnchor: &anchor,
		changedLines: []changedLine{
			testChangedLine(lineRef{File: "a.go", Line: 12, Side: "new"}),
			testChangedLine(lineRef{File: "a.go", Line: 10, Side: "new"}),
		},
	}

	got, err := state.currentRange()
	if err != nil {
		t.Fatal(err)
	}
	if got.Start.Line != 10 || got.End.Line != 12 {
		t.Fatalf("range = %#v, want 10-12", got)
	}
}

func TestSelectSideSwitchesReviewTargetOnTwoSidedRow(t *testing.T) {
	left := lineRef{File: "a.go", Line: 10, Side: "old"}
	right := lineRef{File: "a.go", Line: 10, Side: "new"}
	state := &reviewState{
		changedLines: []changedLine{{
			Ref:   right,
			Left:  &left,
			Right: &right,
			Split: 24,
		}},
	}

	state.selectSide("old")
	if state.current().Side != "old" || state.current().Line != 10 {
		t.Fatalf("current after h = %#v, want old side", state.current())
	}
	if state.message != "" {
		t.Fatalf("side switch message = %q, want no message", state.message)
	}
	start, end := changedLineHighlightRange(state.changedLines[0], 80)
	if start != 0 || end != 24 {
		t.Fatalf("old highlight range = %d-%d, want 0-24", start, end)
	}

	state.selectSide("new")
	if state.current().Side != "new" || state.current().Line != 10 {
		t.Fatalf("current after l = %#v, want new side", state.current())
	}
	if state.message != "" {
		t.Fatalf("side switch message = %q, want no message", state.message)
	}
	start, end = changedLineHighlightRange(state.changedLines[0], 80)
	if start != 24 || end != 80 {
		t.Fatalf("new highlight range = %d-%d, want 24-80", start, end)
	}
}

func TestRestoreCursorKeepsSameLineAndSide(t *testing.T) {
	left := lineRef{File: "a.go", Line: 10, Side: "old"}
	right := lineRef{File: "a.go", Line: 10, Side: "new"}
	state := &reviewState{
		cursor: 1,
		top:    4,
		changedLines: []changedLine{
			testChangedLine(lineRef{File: "a.go", Line: 9, Side: "new"}),
			{Ref: right, Left: &left, Right: &right},
			testChangedLine(lineRef{File: "b.go", Line: 1, Side: "new"}),
		},
	}

	state.restoreCursor(lineRef{File: "a.go", Line: 10, Side: "old"})

	if state.cursor != 1 {
		t.Fatalf("cursor = %d, want restored row 1", state.cursor)
	}
	if state.current().Side != "old" || state.current().Line != 10 {
		t.Fatalf("current = %#v, want old side line 10", state.current())
	}
}

func TestSelectionMovementKeepsAnchorSideOnTwoSidedRows(t *testing.T) {
	firstLeft := lineRef{File: "a.go", Line: 10, Side: "old"}
	firstRight := lineRef{File: "a.go", Line: 10, Side: "new"}
	secondLeft := lineRef{File: "a.go", Line: 11, Side: "old"}
	secondRight := lineRef{File: "a.go", Line: 11, Side: "new"}
	state := &reviewState{
		source: sourceBranch,
		draft:  reviewDraft{Active: true},
		changedLines: []changedLine{
			{Ref: firstRight, Left: &firstLeft, Right: &firstRight},
			{Ref: secondRight, Left: &secondLeft, Right: &secondRight},
		},
	}

	state.selectSide("old")
	state.toggleSelection()
	state.move(1)

	if state.cursor != 1 {
		t.Fatalf("cursor = %d, want 1", state.cursor)
	}
	if state.current().Side != "old" || state.current().Line != 11 {
		t.Fatalf("current = %#v, want second old side", state.current())
	}
}

func TestReviewSuggestionRejectsOldSide(t *testing.T) {
	state := &reviewState{
		source: sourceBranch,
		pr:     &prContext{Head: "head", Base: "base"},
		draft:  reviewDraft{Active: true, ID: "review-id"},
		changedLines: []changedLine{
			testChangedLine(lineRef{File: "a.go", Line: 10, Side: "old"}),
		},
	}

	err := state.reviewSuggestion(&terminalState{})
	if err == nil {
		t.Fatal("expected old-side suggestion to be rejected")
	}
	if !strings.Contains(err.Error(), "right side") {
		t.Fatalf("error = %q, want right-side message", err)
	}
}

func TestToggleSelectionRequiresBranchDraftReview(t *testing.T) {
	state := &reviewState{
		source: sourceLocal,
		draft:  reviewDraft{Active: true},
		changedLines: []changedLine{
			testChangedLine(lineRef{File: "a.go", Line: 10, Side: "new"}),
		},
	}

	state.toggleSelection()
	if state.selectionAnchor != nil {
		t.Fatal("expected local source to reject multi-line selection")
	}
	if !strings.Contains(state.message, "reviewing branch changes") {
		t.Fatalf("message = %q, want reviewing branch changes hint", state.message)
	}

	state.source = sourceBranch
	state.draft = reviewDraft{Active: true}
	state.message = ""
	state.toggleSelection()
	if state.selectionAnchor != nil {
		t.Fatal("expected branch source without PR to reject multi-line selection")
	}
	if !strings.Contains(state.message, "No active GitHub PR") {
		t.Fatalf("message = %q, want no-PR hint", state.message)
	}

	state.pr = &prContext{Head: "head", Base: "base"}
	state.draft = reviewDraft{}
	state.message = ""
	state.toggleSelection()
	if state.selectionAnchor != nil {
		t.Fatal("expected branch source without draft review to reject multi-line selection")
	}
	if !strings.Contains(state.message, "No draft review active") {
		t.Fatalf("message = %q, want no-draft hint", state.message)
	}

	state.draft = reviewDraft{Active: true}
	state.toggleSelection()
	if state.selectionAnchor == nil {
		t.Fatal("expected branch source with draft review to start multi-line selection")
	}
}

func TestReviewActionsRequireDraft(t *testing.T) {
	state := &reviewState{
		source: sourceBranch,
		pr:     &prContext{Head: "head", Base: "base"},
		changedLines: []changedLine{
			testChangedLine(lineRef{File: "a.go", Line: 10, Side: "new"}),
		},
	}

	err := state.reviewComment(&terminalState{})
	if err == nil || !strings.Contains(err.Error(), "No draft review active") {
		t.Fatalf("comment error = %v, want no-draft error", err)
	}

	err = state.reviewSuggestion(&terminalState{})
	if err == nil || !strings.Contains(err.Error(), "No draft review active") {
		t.Fatalf("suggestion error = %v, want no-draft error", err)
	}
}

func TestReviewActionsRequireBranchChanges(t *testing.T) {
	state := &reviewState{
		source: sourceLocal,
		pr:     &prContext{Head: "head", Base: "base"},
		draft:  reviewDraft{Active: true, ID: "review-id"},
		changedLines: []changedLine{
			testChangedLine(lineRef{File: "a.go", Line: 10, Side: "new"}),
		},
	}

	state.startReview()
	if !strings.Contains(state.message, "reviewing branch changes") {
		t.Fatalf("start review message = %q, want branch-changes guard", state.message)
	}

	err := state.reviewComment(&terminalState{})
	if err == nil || !strings.Contains(err.Error(), "reviewing branch changes") {
		t.Fatalf("comment error = %v, want branch-changes guard", err)
	}

	err = state.reviewSuggestion(&terminalState{})
	if err == nil || !strings.Contains(err.Error(), "reviewing branch changes") {
		t.Fatalf("suggestion error = %v, want branch-changes guard", err)
	}

	err = state.submitReview(&terminalState{}, github.ReviewComment)
	if err == nil || !strings.Contains(err.Error(), "reviewing branch changes") {
		t.Fatalf("submit error = %v, want branch-changes guard", err)
	}

	state.message = ""
	state.discardReview()
	if !strings.Contains(state.message, "reviewing branch changes") {
		t.Fatalf("delete draft message = %q, want branch-changes guard", state.message)
	}
}

func TestReviewabilityFollowsDiffArgs(t *testing.T) {
	local := newReviewState(nil)
	if local.canReviewBranchChanges() {
		t.Fatal("plain local diff should not be reviewable")
	}

	branch := newReviewState([]string{"main...HEAD"})
	if !branch.canReviewBranchChanges() {
		t.Fatal("triple-dot branch diff should be reviewable")
	}

	staged := newReviewState([]string{"--staged"})
	if staged.canReviewBranchChanges() {
		t.Fatal("staged local diff should not be reviewable")
	}
}

func TestSubmitReviewKeys(t *testing.T) {
	state := &reviewState{
		reviewable: true,
	}

	state.handleKey("R", &terminalState{}, 8)
	if state.message != "" {
		t.Fatalf("inactive draft R message = %q, want no action", state.message)
	}

	for _, key := range []string{"A", "C"} {
		state.message = ""
		state.handleKey(key, &terminalState{}, 8)
		if !strings.Contains(state.message, "No active GitHub PR") {
			t.Fatalf("%s message = %q, want submit-review path", key, state.message)
		}
	}
}

func TestSubmitReviewAllowsEmptyBody(t *testing.T) {
	events := []github.ReviewEvent{
		github.ReviewComment,
		github.ReviewApprove,
		github.ReviewRequestChanges,
	}

	for _, event := range events {
		t.Run(string(event), func(t *testing.T) {
			dir := t.TempDir()
			inputPath := dir + "/gh-input.json"
			scriptPath := dir + "/gh"
			script := "#!/bin/sh\ncat > " + shellQuote(inputPath) + "\nprintf '%s\\n' '{\"data\":{\"submitPullRequestReview\":{\"pullRequestReview\":{\"id\":\"review-id\"}}}}'\n"
			if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
				t.Fatal(err)
			}

			t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
			t.Setenv("VISUAL", "")
			t.Setenv("EDITOR", "true")

			state := &reviewState{
				source: sourceBranch,
				pr:     &prContext{ID: "pr-id"},
				draft:  reviewDraft{Active: true, ID: "review-id"},
			}

			if err := state.submitReview(&terminalState{}, event); err != nil {
				t.Fatal(err)
			}
			if state.draft.Active {
				t.Fatal("draft remained active after submit")
			}

			input, err := os.ReadFile(inputPath)
			if err != nil {
				t.Fatal(err)
			}
			var request struct {
				Variables map[string]any `json:"variables"`
			}
			if err := json.Unmarshal(input, &request); err != nil {
				t.Fatal(err)
			}
			if request.Variables["body"] != nil {
				t.Fatalf("body = %#v, want nil", request.Variables["body"])
			}
			if request.Variables["event"] != string(event) {
				t.Fatalf("event = %#v, want %q", request.Variables["event"], event)
			}
		})
	}
}

func TestStartAndRequestChangesKeys(t *testing.T) {
	state := &reviewState{
		reviewable: true,
	}

	state.handleKey("P", &terminalState{}, 8)
	if !strings.Contains(state.message, "No active GitHub PR") {
		t.Fatalf("P message = %q, want start-review path", state.message)
	}

	state.draft = reviewDraft{Active: true}
	state.message = ""
	state.handleKey("R", &terminalState{}, 8)
	if !strings.Contains(state.message, "No active GitHub PR") {
		t.Fatalf("active draft R message = %q, want submit-review path", state.message)
	}
}

func TestOwnPRDecisionReviewKeysAreIgnored(t *testing.T) {
	state := &reviewState{
		reviewable: true,
		pr:         &prContext{Number: 4, Author: "octo", Viewer: "Octo"},
		draft:      reviewDraft{Active: true, ID: "review-id"},
	}

	state.handleKey("A", &terminalState{}, 8)
	if !strings.Contains(state.message, "Cannot approve your own pull request") {
		t.Fatalf("A message = %q, want own-PR guard", state.message)
	}

	state.message = ""
	state.handleKey("R", &terminalState{}, 8)
	if !strings.Contains(state.message, "Cannot request changes on your own pull request") {
		t.Fatalf("R message = %q, want own-PR guard", state.message)
	}
}

func testChangedLine(ref lineRef) changedLine {
	row := changedLine{Ref: ref}
	row.setSideRef(ref)
	return row
}

func TestReviewCommentPayloadOmitsStartForSingleLine(t *testing.T) {
	pr := &prContext{Head: "abc123"}
	reviewRange := reviewRange{
		Start: lineRef{File: "a.go", Line: 5, Side: "new"},
		End:   lineRef{File: "a.go", Line: 5, Side: "new"},
	}

	got := github.ReviewCommentPayload(pr, githubRange(reviewRange), "body")
	if got["line"] != 5 || got["side"] != "RIGHT" || got["commit_id"] != "abc123" {
		t.Fatalf("payload = %#v", got)
	}
	if _, ok := got["start_line"]; ok {
		t.Fatalf("single-line payload included start_line: %#v", got)
	}
}

func TestReviewCommentPayloadIncludesStartForMultiLineOldSide(t *testing.T) {
	pr := &prContext{Head: "abc123"}
	reviewRange := reviewRange{
		Start: lineRef{File: "a.go", Line: 5, Side: "old"},
		End:   lineRef{File: "a.go", Line: 7, Side: "old"},
	}

	got := github.ReviewCommentPayload(pr, githubRange(reviewRange), "body")
	if got["line"] != 7 || got["side"] != "LEFT" || got["start_line"] != 5 || got["start_side"] != "LEFT" {
		t.Fatalf("payload = %#v", got)
	}
}

func TestReviewThreadVariablesIncludesRange(t *testing.T) {
	reviewRange := reviewRange{
		Start: lineRef{File: "a.go", Line: 5, Side: "new"},
		End:   lineRef{File: "a.go", Line: 7, Side: "new"},
	}

	got := github.ReviewThreadVariables("review-id", githubRange(reviewRange), "body")
	if got["reviewID"] != "review-id" || got["path"] != "a.go" || got["line"] != 7 || got["side"] != "RIGHT" {
		t.Fatalf("variables = %#v", got)
	}
	if got["startLine"] != 5 || got["startSide"] != "RIGHT" || got["body"] != "body" {
		t.Fatalf("variables = %#v", got)
	}
}

func TestReviewThreadVariablesOmitsStartForSingleLine(t *testing.T) {
	reviewRange := reviewRange{
		Start: lineRef{File: "a.go", Line: 5, Side: "old"},
		End:   lineRef{File: "a.go", Line: 5, Side: "old"},
	}

	got := github.ReviewThreadVariables("review-id", githubRange(reviewRange), "body")
	if got["line"] != 5 || got["side"] != "LEFT" {
		t.Fatalf("variables = %#v", got)
	}
	if _, ok := got["startLine"]; ok {
		t.Fatalf("single-line variables included startLine: %#v", got)
	}
}

func TestSuggestionFenceUsesTripleBackticksForPlainContent(t *testing.T) {
	got := suggestionFence("plain content")

	if got != "```" {
		t.Fatalf("fence = %q, want triple backticks", got)
	}
}

func TestSuggestionFenceOutrunsMarkdownFences(t *testing.T) {
	got := suggestionFence("before\n```md\n# title\n```\nafter")

	if got != "````" {
		t.Fatalf("fence = %q, want four backticks", got)
	}
}

func TestReceiveReviewContextUsesAsyncResult(t *testing.T) {
	ch := make(chan reviewContext, 1)
	ch <- reviewContext{
		PR:    &prContext{Number: 4},
		Draft: reviewDraft{Active: true, ID: "review-id", Count: 2},
	}
	state := &reviewState{prChecking: true}

	state.receiveReviewContext(ch)
	if state.prChecking {
		t.Fatal("expected PR check to complete")
	}
	if state.pr == nil || state.pr.Number != 4 {
		t.Fatalf("pr = %#v, want PR #4", state.pr)
	}
	if !state.draft.Active || state.draft.ID != "review-id" || state.draft.Count != 2 {
		t.Fatalf("draft = %#v, want detected draft review", state.draft)
	}
}

func TestReceiveReviewContextDoesNotBlockWhenPending(t *testing.T) {
	ch := make(chan reviewContext)
	state := &reviewState{prChecking: true}

	state.receiveReviewContext(ch)
	if !state.prChecking {
		t.Fatal("expected PR check to remain pending")
	}
}

func TestRequirePRReportsPendingCheck(t *testing.T) {
	state := &reviewState{prChecking: true}

	err := state.requirePR("start review")
	if err == nil || !strings.Contains(err.Error(), "Checking for an active GitHub PR") {
		t.Fatalf("error = %v, want pending-check message", err)
	}
}
