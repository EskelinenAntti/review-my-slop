package app

import (
	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"slices"
	"testing"
)

func TestCommentAnchorUsesSemanticSelection(t *testing.T) {
	lines := []patch.Line{
		{Kind: patch.Context, Text: "before", OldNumber: 1, NewNumber: 1},
		{Kind: patch.Deletion, Text: "removed", OldNumber: 2},
		{Kind: patch.Addition, Text: "added", NewNumber: 2},
	}
	anchor := commentAnchor(patch.File{OldPath: "old.go", NewPath: "new.go"}, lines)
	if anchor.FilePath != "new.go" || anchor.OldStart != 1 || anchor.OldEnd != 2 || anchor.NewStart != 1 || anchor.NewEnd != 2 || !slices.Equal(anchor.QuotedLines, []string{" before", "-removed", "+added"}) {
		t.Fatalf("anchor=%#v", anchor)
	}
	anchor = commentAnchor(patch.File{OldPath: "deleted.go"}, lines[1:2])
	if anchor.FilePath != "deleted.go" || anchor.NewStart != 0 {
		t.Fatalf("deleted anchor=%#v", anchor)
	}
}
