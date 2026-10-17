package app

import (
	"reflect"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/comments"
	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

func TestCommentAnchorUsesSemanticSelection(t *testing.T) {
	lines := []patch.Line{
		{Kind: patch.Context, Text: "before", OldNumber: 1, NewNumber: 1},
		{Kind: patch.Deletion, Text: "removed", OldNumber: 2},
		{Kind: patch.Addition, Text: "added", NewNumber: 2},
	}
	anchor := commentAnchor(
		patch.File{OldPath: "old.go", NewPath: "new.go"},
		lines,
	)
	want := comments.Anchor{
		FilePath:    "new.go",
		OldStart:    1,
		OldEnd:      2,
		NewStart:    1,
		NewEnd:      2,
		QuotedLines: []string{" before", "-removed", "+added"},
	}
	if !reflect.DeepEqual(anchor, want) {
		t.Fatalf("anchor=%#v, want %#v", anchor, want)
	}
	anchor = commentAnchor(patch.File{OldPath: "deleted.go"}, lines[1:2])
	if anchor.FilePath != "deleted.go" || anchor.NewStart != 0 {
		t.Fatalf("deleted anchor=%#v", anchor)
	}
}
