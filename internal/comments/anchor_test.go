package comments

import (
	"slices"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

func TestNewAnchorUsesSemanticSelection(t *testing.T) {
	lines := []patch.Line{
		{Kind: patch.Context, Text: "before", OldNumber: 1, NewNumber: 1},
		{Kind: patch.Deletion, Text: "removed", OldNumber: 2},
		{Kind: patch.Addition, Text: "added", NewNumber: 2},
	}
	anchor := NewAnchor(
		patch.File{OldPath: "old.go", NewPath: "new.go"},
		lines,
	)
	if anchor.FilePath != "new.go" ||
		anchor.OldStart != 1 || anchor.OldEnd != 2 ||
		anchor.NewStart != 1 || anchor.NewEnd != 2 || !slices.Equal(
		anchor.QuotedLines,
		[]string{" before", "-removed", "+added"},
	) {
		t.Fatalf("anchor=%#v", anchor)
	}
	anchor = NewAnchor(patch.File{OldPath: "deleted.go"}, lines[1:2])
	if anchor.FilePath != "deleted.go" || anchor.NewStart != 0 {
		t.Fatalf("deleted anchor=%#v", anchor)
	}
}

func TestNewAnchorRangeBounds(t *testing.T) {
	file := patch.File{NewPath: "file.go"}
	cases := []struct {
		name  string
		lines []patch.Line
		want  Anchor
	}{
		{
			name: "reverse order",
			lines: []patch.Line{
				{
					Kind: patch.Context, Text: "last",
					OldNumber: 8, NewNumber: 10,
				},
				{
					Kind: patch.Context, Text: "first",
					OldNumber: 3, NewNumber: 5,
				},
			},
			want: Anchor{
				FilePath: "file.go", OldStart: 3, OldEnd: 8,
				NewStart: 5, NewEnd: 10,
				QuotedLines: []string{" last", " first"},
			},
		},
		{
			name: "addition only",
			lines: []patch.Line{{
				Kind: patch.Addition, Text: "added", NewNumber: 4,
			}},
			want: Anchor{
				FilePath: "file.go", NewStart: 4, NewEnd: 4,
				QuotedLines: []string{"+added"},
			},
		},
		{
			name: "deletion only",
			lines: []patch.Line{{
				Kind: patch.Deletion, Text: "removed", OldNumber: 7,
			}},
			want: Anchor{
				FilePath: "file.go", OldStart: 7, OldEnd: 7,
				QuotedLines: []string{"-removed"},
			},
		},
		{
			name: "empty selection",
			want: Anchor{FilePath: "file.go"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := NewAnchor(file, tc.lines)
			if got.FilePath != tc.want.FilePath ||
				got.OldStart != tc.want.OldStart ||
				got.OldEnd != tc.want.OldEnd ||
				got.NewStart != tc.want.NewStart ||
				got.NewEnd != tc.want.NewEnd ||
				!slices.Equal(got.QuotedLines, tc.want.QuotedLines) {
				t.Fatalf("NewAnchor() = %#v, want %#v", got, tc.want)
			}
		})
	}
}
