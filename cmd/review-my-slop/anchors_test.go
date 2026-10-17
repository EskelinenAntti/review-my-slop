package main

import (
	"slices"
	"testing"
)

func TestAppCommentAnchorUsesSemanticSelection(t *testing.T) {
	lines := []patchLine{
		{Kind: patchContext, Text: "before", OldNumber: 1, NewNumber: 1},
		{Kind: patchDeletion, Text: "removed", OldNumber: 2},
		{Kind: patchAddition, Text: "added", NewNumber: 2},
	}
	anchor := testAnchorForLines(patchFile{
		OldPath: "old.go",
		NewPath: "new.go",
	}, lines)
	wantLines := []string{" before", "-removed", "+added"}
	if anchor.FilePath != "new.go" || anchor.OldStart != 1 ||
		anchor.OldEnd != 2 ||
		anchor.NewStart != 1 || anchor.NewEnd != 2 ||
		!slices.Equal(anchor.QuotedLines, wantLines) {
		t.Fatalf("anchor=%#v", anchor)
	}
	anchor = testAnchorForLines(patchFile{OldPath: "deleted.go"}, lines[1:2])
	if anchor.FilePath != "deleted.go" || anchor.NewStart != 0 {
		t.Fatalf("deleted anchor=%#v", anchor)
	}
}

func testAnchorForLines(file patchFile, lines []patchLine) commentsAnchor {
	path := file.NewPath
	if path == "" {
		path = file.OldPath
	}
	anchor := commentsAnchor{FilePath: path}
	for _, line := range lines {
		prefix := " "
		if line.Kind == patchAddition {
			prefix = "+"
		}
		if line.Kind == patchDeletion {
			prefix = "-"
		}
		anchor.QuotedLines = append(anchor.QuotedLines, prefix+line.Text)
		includeTestAnchorLine(&anchor.OldStart, &anchor.OldEnd, int(
			line.OldNumber,
		))
		includeTestAnchorLine(&anchor.NewStart, &anchor.NewEnd, int(
			line.NewNumber,
		))
	}
	return anchor
}

func includeTestAnchorLine(start, end *int, number int) {
	if number == 0 {
		return
	}
	if *start == 0 || number < *start {
		*start = number
	}
	if number > *end {
		*end = number
	}
}
