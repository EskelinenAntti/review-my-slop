package diffscreen

import (
	"slices"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

func TestHeaderSearchFocusesCodeInMatchingFile(t *testing.T) {
	for _, split := range []bool{false, true} {
		v := newDiffView(testPatch(), split, true)
		first, _ := v.first()
		for _, d := range []direction{forward, backward} {
			cursor, ok := v.search("second.go", first, d)
			file, _ := v.file(cursor)
			if !ok || file.DisplayPath != "second.go" {
				t.Fatalf("split=%v direction=%v: cursor=%+v file=%+v", split, d, cursor, file)
			}
		}
	}
}

func TestHeaderSearchUsesOtherPaneAndSkipsFilesWithoutCode(t *testing.T) {
	p := patch.Patch{Files: []patch.File{
		{DisplayPath: "start", Hunks: []patch.Hunk{{Lines: []patch.Line{{Kind: patch.Context, Text: "start", OldNumber: 1, NewNumber: 1}}}}},
		{DisplayPath: "empty target"},
		{DisplayPath: "deletion target", Hunks: []patch.Hunk{{Lines: []patch.Line{{Kind: patch.Deletion, Text: "old", OldNumber: 1}}}}},
	}}
	v := newDiffView(p, true, true)
	first, _ := v.first()
	cursor, ok := v.search("deletion target", first, forward)
	line, _ := v.line(cursor)
	if !ok || cursor.pane != left || line.Text != "old" {
		t.Fatalf("cursor=%+v line=%+v", cursor, line)
	}
	if _, ok := v.search("empty target", first, forward); ok {
		t.Fatal("header search crossed into another file")
	}
}

func TestCrossPaneSelectionIncludesBothChangedLinesAndDeduplicatesContext(t *testing.T) {
	v := newDiffView(testPatch(), true, true)
	first, _ := v.first()
	added, _ := v.search("added one", first, forward)
	selection := diffSelection{First: diffCursor{row: added.row, pane: left}, Last: added}
	got := v.lines(selection)
	if len(got) != 2 || got[0].Kind != patch.Deletion || got[1].Kind != patch.Addition {
		t.Fatalf("lines=%+v", got)
	}
	for _, pane := range []diffPane{left, right} {
		if !selected(&selection, diffCursor{row: added.row, pane: pane}) {
			t.Fatalf("pane %v not selected", pane)
		}
	}
	if selected(&selection, first) {
		t.Fatal("selection includes another row")
	}
	selection = diffSelection{First: diffCursor{row: first.row, pane: left}, Last: first}
	if got := v.lines(selection); len(got) != 1 || got[0].Text != "before" {
		t.Fatalf("context lines=%+v", got)
	}
}

func TestPaneSwitchTranslatesActiveSelection(t *testing.T) {
	v := New(testPatch(), Options{SideBySide: true, Dark: true})
	v.Resize(120, 30)
	v.Move(NextLine)
	v.ToggleSelection()
	v.Move(NextLine)
	v.Move(OldPane)
	_, lines, ok := v.Selected()
	if !ok || len(lines) != 3 || lines[0].Kind != patch.Deletion || lines[1].Kind != patch.Deletion || lines[2].Kind != patch.Context {
		t.Fatalf("old selection=%+v", lines)
	}
	v.Move(NewPane)
	_, lines, ok = v.Selected()
	if !ok || !slices.ContainsFunc(lines, func(line patch.Line) bool { return line.Kind == patch.Addition }) {
		t.Fatalf("new selection=%+v", lines)
	}
	v.ClearSelection()
	v.Move(OldPane)
	if v.selection != nil || v.cursor.pane != left {
		t.Fatalf("cursor=%+v selection=%+v", v.cursor, v.selection)
	}
}

func TestFindRejectsInvalidDirectionWithoutMoving(t *testing.T) {
	v := New(testPatch(), Options{})
	v.BeginSearch()
	v.InsertSearch("added")
	v.AcceptSearch()
	before := v.cursor
	if v.Find(Direction(0)) || v.cursor != before {
		t.Fatal("invalid direction moved cursor")
	}
}
