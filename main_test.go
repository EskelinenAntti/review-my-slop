package main

import (
	"strings"
	"testing"
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

func TestTruncateANSIPreservesEscapeSequences(t *testing.T) {
	got := truncateANSI("\x1b[31mabcdef\x1b[0m", 4)
	if !ansiRE.MatchString(got) {
		t.Fatalf("expected ansi escapes in %q", got)
	}
	if visibleLen(got) != 4 {
		t.Fatalf("visible length = %d, want 4", visibleLen(got))
	}
}

func TestBuildSelectionsUsesDisplayedRows(t *testing.T) {
	lines := []string{
		"\x1b[1mmain.go\x1b[0m --- Go",
		"\x1b[2m 10 \x1b[0m old text        \x1b[2m 10 \x1b[0m new text",
		"\x1b[2m .. \x1b[0m wrapped text     \x1b[2m .. \x1b[0m wrapped text",
		"\x1b[2m .. \x1b[0m                  \x1b[92;1m 12 \x1b[0m added only",
		"\x1b[91;1m 11 \x1b[0m removed       \x1b[92;1m 11 \x1b[0m added",
	}

	selections := buildSelections(lines, nil)
	if len(selections) != 2 {
		t.Fatalf("expected 2 selectable rows, got %d", len(selections))
	}
	if selections[0].LineIndex != 3 || selections[0].Ref.Line != 12 || selections[0].Ref.Side != "new" {
		t.Fatalf("first selection = %#v", selections[0])
	}
	if selections[1].LineIndex != 4 || selections[1].Ref.Line != 11 || selections[1].Ref.Side != "new" {
		t.Fatalf("second selection = %#v", selections[1])
	}
	if selections[1].Left == nil || selections[1].Left.Line != 11 || selections[1].Left.Side != "old" {
		t.Fatalf("second selection left side = %#v", selections[1].Left)
	}
	if selections[1].Right == nil || selections[1].Right.Line != 11 || selections[1].Right.Side != "new" {
		t.Fatalf("second selection right side = %#v", selections[1].Right)
	}
}

func TestHighlightPlainStripsDiffColors(t *testing.T) {
	got := highlightPlain("\x1b[31mred\x1b[0m plain", 12)
	if strings.Contains(got, "\x1b[31m") {
		t.Fatalf("expected diff color to be stripped from %q", got)
	}
	if visibleLen(got) != 12 {
		t.Fatalf("visible length = %d, want 12", visibleLen(got))
	}
}

func TestSelectionMovementStaysWithinFileAndSide(t *testing.T) {
	state := &reviewState{
		selections: []displaySelection{
			testSelection(lineRef{File: "a.go", Line: 10, Side: "new"}),
			testSelection(lineRef{File: "a.go", Line: 11, Side: "new"}),
			testSelection(lineRef{File: "a.go", Line: 12, Side: "old"}),
			testSelection(lineRef{File: "b.go", Line: 1, Side: "new"}),
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
		selections: []displaySelection{
			testSelection(lineRef{File: "a.go", Line: 12, Side: "new"}),
			testSelection(lineRef{File: "a.go", Line: 10, Side: "new"}),
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
		selections: []displaySelection{{
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
	start, end := selectionHighlightRange(state.selections[0], 80)
	if start != 0 || end != 24 {
		t.Fatalf("old highlight range = %d-%d, want 0-24", start, end)
	}

	state.selectSide("new")
	if state.current().Side != "new" || state.current().Line != 10 {
		t.Fatalf("current after l = %#v, want new side", state.current())
	}
	start, end = selectionHighlightRange(state.selections[0], 80)
	if start != 24 || end != 80 {
		t.Fatalf("new highlight range = %d-%d, want 24-80", start, end)
	}
}

func TestSelectionMovementKeepsAnchorSideOnTwoSidedRows(t *testing.T) {
	firstLeft := lineRef{File: "a.go", Line: 10, Side: "old"}
	firstRight := lineRef{File: "a.go", Line: 10, Side: "new"}
	secondLeft := lineRef{File: "a.go", Line: 11, Side: "old"}
	secondRight := lineRef{File: "a.go", Line: 11, Side: "new"}
	state := &reviewState{
		selections: []displaySelection{
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
		pr: &prContext{Head: "head", Base: "base"},
		selections: []displaySelection{
			testSelection(lineRef{File: "a.go", Line: 10, Side: "old"}),
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

func testSelection(ref lineRef) displaySelection {
	selection := displaySelection{Ref: ref}
	selection.setSideRef(ref)
	return selection
}

func TestReviewCommentPayloadOmitsStartForSingleLine(t *testing.T) {
	pr := &prContext{Head: "abc123"}
	reviewRange := reviewRange{
		Start: lineRef{File: "a.go", Line: 5, Side: "new"},
		End:   lineRef{File: "a.go", Line: 5, Side: "new"},
	}

	got := reviewCommentPayload(pr, reviewRange, "body")
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

	got := reviewCommentPayload(pr, reviewRange, "body")
	if got["line"] != 7 || got["side"] != "LEFT" || got["start_line"] != 5 || got["start_side"] != "LEFT" {
		t.Fatalf("payload = %#v", got)
	}
}
