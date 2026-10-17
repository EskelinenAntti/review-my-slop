package app

import (
	"slices"
	"testing"

	patches "github.com/eskelinenantti/review-my-slop/internal/patch"
)

func TestAppCommentAnchorUsesSemanticSelection(t *testing.T) {
	lines := []patches.Line{
		{Kind: patches.Context, Text: "before", OldNumber: 1, NewNumber: 1},
		{Kind: patches.Deletion, Text: "removed", OldNumber: 2},
		{Kind: patches.Addition, Text: "added", NewNumber: 2},
	}
	anchor := selectedCommentAnchor(patches.File{
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
	anchor = selectedCommentAnchor(patches.File{OldPath: "deleted.go"}, lines[1:2])
	if anchor.FilePath != "deleted.go" || anchor.NewStart != 0 {
		t.Fatalf("deleted anchor=%#v", anchor)
	}
}
