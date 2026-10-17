package app

import (
	"slices"
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
	anchor := func() comments.Anchor {
		var file patch.File = patch.File{OldPath: "old.go", NewPath: "new.go"}
		path := file.NewPath
		if path == "" {
			path = file.OldPath
		}
		anchor := comments.Anchor{FilePath: path}
		for _, line := range lines {
			prefix := " "
			if line.Kind == patch.Addition {
				prefix = "+"
			}
			if line.Kind == patch.Deletion {
				prefix = "-"
			}
			anchor.QuotedLines = append(anchor.QuotedLines, prefix+line.Text)
			func() {
				var inlineLocalStart1, end *int = &anchor.OldStart, &anchor.OldEnd
				if int(line.OldNumber) == 0 {
					return
				}
				if *inlineLocalStart1 == 0 || int(line.OldNumber) < *inlineLocalStart1 {
					*inlineLocalStart1 = int(line.OldNumber)
				}
				if int(line.OldNumber) > *end {
					*end = int(line.OldNumber)
				}
			}()
			func() {
				var inlineLocalStart4, end *int = &anchor.NewStart, &anchor.NewEnd
				if int(line.NewNumber) == 0 {
					return
				}
				if *inlineLocalStart4 == 0 || int(line.NewNumber) < *inlineLocalStart4 {
					*inlineLocalStart4 = int(line.NewNumber)
				}
				if int(line.NewNumber) > *end {
					*end = int(line.NewNumber)
				}
			}()
		}
		return anchor
	}()
	if anchor.FilePath != "new.go" ||
		anchor.OldStart != 1 || anchor.OldEnd != 2 ||
		anchor.NewStart != 1 || anchor.NewEnd != 2 || !slices.Equal(

		anchor.QuotedLines,

		[]string{" before", "-removed", "+added"},
	) {
		t.Fatalf("anchor=%#v", anchor)
	}
	anchor = func() comments.Anchor {
		var (
			file  patch.File   = patch.File{OldPath: "deleted.go"}
			lines []patch.Line = lines[1:2]
		)
		path := file.NewPath
		if path == "" {
			path = file.OldPath
		}
		anchor := comments.Anchor{FilePath: path}
		for _, line := range lines {
			prefix := " "
			if line.Kind == patch.Addition {
				prefix = "+"
			}
			if line.Kind == patch.Deletion {
				prefix = "-"
			}
			anchor.QuotedLines = append(anchor.QuotedLines, prefix+line.Text)
			func() {
				var inlineLocalStart2, end *int = &anchor.OldStart, &anchor.OldEnd
				if int(line.OldNumber) == 0 {
					return
				}
				if *inlineLocalStart2 == 0 || int(line.OldNumber) < *inlineLocalStart2 {
					*inlineLocalStart2 = int(line.OldNumber)
				}
				if int(line.OldNumber) > *end {
					*end = int(line.OldNumber)
				}
			}()
			func() {
				var inlineLocalStart3, end *int = &anchor.NewStart, &anchor.NewEnd
				if int(line.NewNumber) == 0 {
					return
				}
				if *inlineLocalStart3 == 0 || int(line.NewNumber) < *inlineLocalStart3 {
					*inlineLocalStart3 = int(line.NewNumber)
				}
				if int(line.NewNumber) > *end {
					*end = int(line.NewNumber)
				}
			}()
		}
		return anchor
	}()
	if anchor.FilePath != "deleted.go" || anchor.NewStart != 0 {
		t.Fatalf("deleted anchor=%#v", anchor)
	}
}
