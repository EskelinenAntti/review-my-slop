package app

import (
	"slices"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

func TestCommentAnchorUsesSemanticSelection(t *testing.T) {
	lines := []patch.Line{
		{Kind: patch.Context, Text: "before", OldNumber: one, NewNumber: one},
		{Kind: patch.Deletion, Text: "removed", OldNumber: two},
		{Kind: patch.Addition, Text: "added", NewNumber: two},
	}
	anchor := commentAnchor(
		patch.File{OldPath: "old.go", NewPath: "new.go"},
		lines,
	)
	requireEqual(
		t,
		anchor.FilePath == "new.go" && anchor.OldStart == one &&
			anchor.OldEnd == two &&
			anchor.NewStart == one &&
			anchor.NewEnd == two &&
			slices.Equal(
				anchor.QuotedLines,
				[]string{" before", "-removed", "+added"},
			),
		true,
		"anchor=%#v",
		anchor,
	)
	anchor = commentAnchor(patch.File{OldPath: "deleted.go"}, lines[one:two])
	requireEqual(
		t,
		anchor.FilePath == "deleted.go" && anchor.NewStart == 0, true,
		"deleted anchor=%#v",
		anchor,
	)
}
