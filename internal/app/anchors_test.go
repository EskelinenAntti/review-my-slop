package app

import (
	"slices"
	"testing"

	feedback "github.com/eskelinenantti/review-my-slop/internal/comments"
	patches "github.com/eskelinenantti/review-my-slop/internal/patch"
)

func TestAppCommentAnchorUsesSemanticSelection(t *testing.T) {
	lines := []patches.Line{
		{Kind: patches.Context, Text: "before", OldNumber: 1, NewNumber: 1},
		{Kind: patches.Deletion, Text: "removed", OldNumber: 2},
		{Kind: patches.Addition, Text: "added", NewNumber: 2},
	}
	anchor := testAnchorForLines(patches.File{
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
	anchor = testAnchorForLines(patches.File{OldPath: "deleted.go"}, lines[1:2])
	if anchor.FilePath != "deleted.go" || anchor.NewStart != 0 {
		t.Fatalf("deleted anchor=%#v", anchor)
	}
}

func testAnchorForLines(
	file patches.File, lines []patches.Line,
) feedback.Anchor {
	path := file.NewPath
	if path == "" {
		path = file.OldPath
	}
	anchor := feedback.Anchor{FilePath: path}
	for _, line := range lines {
		prefix := " "
		if line.Kind == patches.Addition {
			prefix = "+"
		}
		if line.Kind == patches.Deletion {
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
