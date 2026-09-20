package comments

import (
	"reflect"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

func TestAnchorForBuildsCanonicalAnchor(t *testing.T) {
	file := patch.File{OldPath: "old.go", NewPath: "new.go"}
	lines := []patch.Line{
		{Kind: patch.Context, Text: "keep()", OldNumber: 3, NewNumber: 3},
		{Kind: patch.Deletion, Text: "remove()", OldNumber: 4},
		{Kind: patch.Addition, Text: "add()", NewNumber: 4},
	}

	anchor, err := AnchorFor(file, lines)
	if err != nil {
		t.Fatal(err)
	}
	want := Anchor{
		FilePath:    "new.go",
		OldStart:    3,
		OldEnd:      4,
		NewStart:    3,
		NewEnd:      4,
		QuotedLines: []string{" keep()", "-remove()", "+add()"},
	}
	if !reflect.DeepEqual(anchor, want) {
		t.Fatalf("anchor = %#v, want %#v", anchor, want)
	}
}

func TestAnchorForRejectsEmptyPatchRange(t *testing.T) {
	if _, err := AnchorFor(patch.File{NewPath: "main.go"}, nil); err == nil {
		t.Fatal("AnchorFor accepted an empty patch range")
	}
}
