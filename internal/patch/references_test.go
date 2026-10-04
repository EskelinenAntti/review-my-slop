package patch

import (
	"slices"
	"testing"
)

func TestAnchorDeduplicatesAndPreservesPatchOrder(t *testing.T) {
	p := Patch{Files: []File{{NewPath: "new.go", Hunks: []Hunk{{Lines: []Line{
		{Kind: Deletion, Text: "removed", OldNumber: 4},
		{Kind: Addition, Text: "added", NewNumber: 7},
		{Kind: Context, Text: "same", OldNumber: 5, NewNumber: 8},
	}}}}}}
	anchor, err := p.Anchor(Range{File: 0, Hunk: 0, Lines: []int{2, 0, 1, 2}})
	if err != nil {
		t.Fatal(err)
	}
	if anchor.FilePath != "new.go" || !slices.Equal(anchor.QuotedLines, []string{"-removed", "+added", " same"}) {
		t.Fatalf("anchor = %#v", anchor)
	}
	if anchor.OldStart != 4 || anchor.OldEnd != 5 || anchor.NewStart != 7 || anchor.NewEnd != 8 {
		t.Fatalf("anchor ranges = %#v", anchor)
	}
}

func TestPatchReferencesRejectInvalidAndEmptyIndices(t *testing.T) {
	p := Patch{Files: []File{{Hunks: []Hunk{{Lines: []Line{{Kind: Context, OldNumber: 1, NewNumber: 1}}}}}}}
	for _, r := range []Range{{File: 0, Hunk: 0}, {File: -1, Hunk: 0, Lines: []int{0}}, {File: 0, Hunk: 1, Lines: []int{0}}, {File: 0, Hunk: 0, Lines: []int{1}}} {
		if _, err := p.Anchor(r); err == nil {
			t.Fatalf("Anchor(%#v) succeeded", r)
		}
	}
	for _, pos := range []Position{{File: -1}, {File: 0, Hunk: 1}, {File: 0, Hunk: 0, Line: -1}, {File: 0, Hunk: 0, Line: 1}} {
		if _, err := p.SourceLocation(pos); err == nil {
			t.Fatalf("SourceLocation(%#v) succeeded", pos)
		}
	}
}

func TestSourceLocationUsesWorkingTreeFallbackAndCounts(t *testing.T) {
	p := Patch{Repository: "/repo", Files: []File{{OldPath: "old.go", NewPath: "/dev/null", Hunks: []Hunk{{Lines: []Line{
		{Kind: Deletion, OldNumber: 3}, {Kind: Addition, NewNumber: 4},
	}}}}}}
	loc, err := p.SourceLocation(Position{File: 0, Hunk: 0, Line: 0})
	if err != nil {
		t.Fatal(err)
	}
	if loc.Path != "/repo/old.go" || loc.Line != 3 {
		t.Fatalf("location = %#v", loc)
	}
	added, removed := p.Counts()
	if added != 1 || removed != 1 {
		t.Fatalf("counts = (%d, %d)", added, removed)
	}
}
