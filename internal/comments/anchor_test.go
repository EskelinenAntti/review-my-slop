package comments

import (
	"reflect"
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
		anchor.NewStart != 1 || anchor.NewEnd != 2 ||
		!slices.Equal(
			anchor.QuotedLines, []string{" before", "-removed", "+added"},
		) {
		t.Fatalf("anchor=%#v", anchor)
	}
	anchor = NewAnchor(patch.File{OldPath: "deleted.go"}, lines[1:2])
	if anchor.FilePath != "deleted.go" || anchor.NewStart != 0 {
		t.Fatalf("deleted anchor=%#v", anchor)
	}
}

func TestNewAnchorRangesAndQuotes(t *testing.T) {
	for _, tt := range []struct {
		name  string
		file  patch.File
		lines []patch.Line
		want  Anchor
	}{
		{
			name: "empty selection",
			file: patch.File{NewPath: "empty.go"},
			want: Anchor{FilePath: "empty.go"},
		},
		{
			name: "addition has no old range",
			file: patch.File{NewPath: "added.go"},
			lines: []patch.Line{
				{Kind: patch.Addition, Text: "", NewNumber: 4},
			},
			want: Anchor{
				FilePath: "added.go", NewStart: 4, NewEnd: 4,
				QuotedLines: []string{"+"},
			},
		},
		{
			name: "deletion has no new range",
			file: patch.File{OldPath: "deleted.go", NewPath: "/dev/null"},
			lines: []patch.Line{
				{Kind: patch.Deletion, Text: "removed", OldNumber: 7},
			},
			want: Anchor{
				FilePath: "deleted.go", OldStart: 7, OldEnd: 7,
				QuotedLines: []string{"-removed"},
			},
		},
		{
			name: "ranges are independent of selection order",
			file: patch.File{OldPath: "old.go", NewPath: "new.go"},
			lines: []patch.Line{
				{
					Kind: patch.Context, Text: "last",
					OldNumber: 9, NewNumber: 12,
				},
				{Kind: patch.Addition, Text: "added", NewNumber: 11},
				{Kind: patch.Deletion, Text: "removed", OldNumber: 8},
				{
					Kind: patch.Context, Text: "first",
					OldNumber: 7, NewNumber: 10,
				},
			},
			want: Anchor{
				FilePath: "new.go", OldStart: 7, OldEnd: 9,
				NewStart: 10, NewEnd: 12,
				QuotedLines: []string{" last", "+added", "-removed", " first"},
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := NewAnchor(tt.file, tt.lines)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("anchor = %#v, want %#v", got, tt.want)
			}
		})
	}
}
