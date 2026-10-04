package layout

import (
	"reflect"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

func fixture() patch.Patch {
	return patch.Patch{Files: []patch.File{{OldPath: "a.go", NewPath: "a.go", DisplayPath: "a.go", Metadata: []string{"mode change"}, Hunks: []patch.Hunk{{Header: "-1,2 +1,2", Lines: []patch.Line{
		{Kind: patch.Context, Text: "before", OldNumber: 1, NewNumber: 1},
		{Kind: patch.Deletion, Text: "removed one", OldNumber: 2},
		{Kind: patch.Deletion, Text: "removed two", OldNumber: 3},
		{Kind: patch.Addition, Text: "added one", NewNumber: 2},
		{Kind: patch.Context, Text: "after", OldNumber: 4, NewNumber: 3},
	}}}}}}
}

func TestSplitBuildPairsUnequalChangeBlocksAndMapsPanes(t *testing.T) {
	d := Build(fixture(), Split)
	var change []Row
	for i := 0; i < d.RowCount(); i++ {
		r := d.Row(i)
		if r.Kind == LineRow && r.LeftLine >= 1 && r.LeftLine <= 2 || r.Kind == LineRow && r.RightLine == 3 {
			change = append(change, r)
		}
	}
	if len(change) != 2 || change[0].LeftLine != 1 || change[0].RightLine != 3 || change[1].LeftLine != 2 || change[1].RightLine != -1 {
		t.Fatalf("paired change rows = %#v", change)
	}
	left, ok := d.Locate(patch.Position{File: 0, Hunk: 0, Line: 1}, Left)
	if !ok || !d.Valid(left) {
		t.Fatalf("left deletion locate = %#v, %v", left, ok)
	}
	right, ok := d.Locate(patch.Position{File: 0, Hunk: 0, Line: 3}, Right)
	if !ok || right.Row != left.Row {
		t.Fatalf("paired addition locate = %#v, %v", right, ok)
	}
}

func TestSplitDoesNotPairAcrossHunksAndPureDeletionUsesLeftPane(t *testing.T) {
	p := patch.Patch{Files: []patch.File{{DisplayPath: "removed.go", Hunks: []patch.Hunk{
		{Header: "first", Lines: []patch.Line{{Kind: patch.Deletion, Text: "gone", OldNumber: 1}}},
		{Header: "second", Lines: []patch.Line{{Kind: patch.Addition, Text: "new", NewNumber: 1}}},
	}}}}
	d := Build(p, Split)
	var rows []Row
	for i := 0; i < d.RowCount(); i++ {
		if r := d.Row(i); r.Kind == LineRow {
			rows = append(rows, r)
		}
	}
	if len(rows) != 2 || rows[0].LeftLine != 0 || rows[0].RightLine != -1 || rows[1].LeftLine != -1 || rows[1].RightLine != 0 || rows[0].Hunk == rows[1].Hunk {
		t.Fatalf("rows across hunks = %#v", rows)
	}
	if !d.Valid(Cell{Row: rows[0].Hunk + 2, Pane: Left}) { // file header and first hunk header precede the deletion.
		t.Fatal("pure deletion is not selectable in the left pane")
	}
}

func TestRangeKeepsActivePaneForMixedSameRowSelection(t *testing.T) {
	d := Build(fixture(), Split)
	left, _ := d.Locate(patch.Position{File: 0, Hunk: 0, Line: 1}, Left)
	right, _ := d.Locate(patch.Position{File: 0, Hunk: 0, Line: 3}, Right)
	r, err := d.Range(Selection{First: left, Last: right})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(r.Lines, []int{1, 3}) {
		t.Fatalf("range lines = %v", r.Lines)
	}
	if _, err := d.Range(Selection{First: left, Last: Cell{Row: 0, Pane: Left}}); err == nil {
		t.Fatal("accepted a header endpoint")
	}
}

func TestRangeAcrossRowsUsesActivePaneAndRejectsHunkCrossing(t *testing.T) {
	d := Build(fixture(), Unified)
	first, _ := d.Locate(patch.Position{File: 0, Hunk: 0, Line: 0}, Right)
	last, _ := d.Locate(patch.Position{File: 0, Hunk: 0, Line: 4}, Right)
	r, err := d.Range(Selection{First: first, Last: last})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(r.Lines, []int{0, 1, 2, 3, 4}) {
		t.Fatalf("unified range lines = %v", r.Lines)
	}

	split := Build(fixture(), Split)
	left, _ := split.Locate(patch.Position{File: 0, Hunk: 0, Line: 1}, Left)
	context, _ := split.Locate(patch.Position{File: 0, Hunk: 0, Line: 4}, Right)
	r, err = split.Range(Selection{First: left, Last: context})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(r.Lines, []int{1, 2, 4}) {
		t.Fatalf("mixed pane range lines = %v", r.Lines)
	}

	multi := patch.Patch{Files: []patch.File{{DisplayPath: "two hunks", Hunks: []patch.Hunk{
		{Header: "one", Lines: []patch.Line{{Kind: patch.Context, Text: "one", OldNumber: 1, NewNumber: 1}}},
		{Header: "two", Lines: []patch.Line{{Kind: patch.Context, Text: "two", OldNumber: 2, NewNumber: 2}}},
	}}}}
	d = Build(multi, Unified)
	a, _ := d.Locate(patch.Position{File: 0, Hunk: 0, Line: 0}, Right)
	b, _ := d.Locate(patch.Position{File: 0, Hunk: 1, Line: 0}, Right)
	if _, err := d.Range(Selection{First: a, Last: b}); err == nil {
		t.Fatal("selection crossed hunk boundary")
	}
	multi.Files = append(multi.Files, patch.File{DisplayPath: "other", Hunks: []patch.Hunk{{Header: "one", Lines: []patch.Line{{Kind: patch.Context, Text: "other", OldNumber: 1, NewNumber: 1}}}}})
	d = Build(multi, Unified)
	a, _ = d.Locate(patch.Position{File: 0, Hunk: 0, Line: 0}, Right)
	b, _ = d.Locate(patch.Position{File: 1, Hunk: 0, Line: 0}, Right)
	if _, err := d.Range(Selection{First: a, Last: b}); err == nil {
		t.Fatal("selection crossed file boundary")
	}
}

func TestFindWrapsAndMatchesPlainHeadersAndLines(t *testing.T) {
	d := Build(fixture(), Unified)
	first, _ := d.Locate(patch.Position{File: 0, Hunk: 0, Line: 0}, Right)
	match, ok := d.Find("ADDED ONE", first, Forward)
	if !ok {
		t.Fatal("line search did not find match")
	}
	pos, _ := d.Position(match)
	if pos.Line != 3 {
		t.Fatalf("match position = %#v", pos)
	}
	if _, ok := d.Find("a.go", first, Forward); !ok {
		t.Fatal("wrapped file header search did not find nearby line")
	}
}

func TestDocumentPatchAndRowsAreDetached(t *testing.T) {
	p := fixture()
	d := Build(p, Unified)
	p.Files[0].Hunks[0].Lines[0].Text = "changed"
	copyPatch := d.Patch()
	copyPatch.Files[0].Hunks[0].Lines[0].Text = "changed again"
	if got := d.Patch().Files[0].Hunks[0].Lines[0].Text; got != "before" {
		t.Fatalf("document patch text = %q", got)
	}
}
