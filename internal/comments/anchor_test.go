package comments

import (
	"reflect"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

func TestNewAnchor(t *testing.T) {
	for _, tt := range []struct {
		name  string
		file  patch.File
		lines []patch.Line
		want  Anchor
	}{
		{
			name: "mixed selection in renamed file",
			file: patch.File{OldPath: "old.go", NewPath: "new.go"},
			lines: []patch.Line{
				{
					Kind: patch.Context, Text: "before",
					OldNumber: 1, NewNumber: 1,
				},
				{Kind: patch.Deletion, Text: "removed", OldNumber: 2},
				{Kind: patch.Addition, Text: "added", NewNumber: 2},
			},
			want: Anchor{
				FilePath: "new.go",
				OldStart: 1, OldEnd: 2, NewStart: 1, NewEnd: 2,
				QuotedLines: []string{" before", "-removed", "+added"},
			},
		},
		{
			name: "deleted file",
			file: patch.File{OldPath: "deleted.go"},
			lines: []patch.Line{
				{Kind: patch.Deletion, Text: "removed", OldNumber: 2},
			},
			want: Anchor{
				FilePath: "deleted.go", OldStart: 2, OldEnd: 2,
				QuotedLines: []string{"-removed"},
			},
		},
		{
			name: "deleted file with dev null path",
			file: patch.File{OldPath: "deleted.go", NewPath: "/dev/null"},
			lines: []patch.Line{
				{Kind: patch.Deletion, Text: "removed", OldNumber: 2},
			},
			want: Anchor{
				FilePath: "deleted.go", OldStart: 2, OldEnd: 2,
				QuotedLines: []string{"-removed"},
			},
		},
		{
			name: "added file",
			file: patch.File{NewPath: "added.go"},
			lines: []patch.Line{
				{Kind: patch.Addition, Text: "added", NewNumber: 3},
			},
			want: Anchor{
				FilePath: "added.go", NewStart: 3, NewEnd: 3,
				QuotedLines: []string{"+added"},
			},
		},
		{
			name: "empty selection",
			file: patch.File{NewPath: "empty.go"},
			want: Anchor{FilePath: "empty.go"},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := NewAnchor(tt.file, tt.lines)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("NewAnchor() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestNewAnchorPreservesQuoteOrderWithUnorderedRanges(t *testing.T) {
	lines := []patch.Line{
		{Kind: patch.Context, Text: "last", OldNumber: 9, NewNumber: 10},
		{Kind: patch.Context, Text: "first", OldNumber: 2, NewNumber: 3},
		{Kind: patch.Context, Text: "middle", OldNumber: 5, NewNumber: 6},
	}
	want := Anchor{
		FilePath: "file.go",
		OldStart: 2, OldEnd: 9, NewStart: 3, NewEnd: 10,
		QuotedLines: []string{" last", " first", " middle"},
	}
	got := NewAnchor(patch.File{NewPath: "file.go"}, lines)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("NewAnchor() = %#v, want %#v", got, want)
	}
}
