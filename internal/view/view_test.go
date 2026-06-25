package view

import (
	"reflect"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

func TestCursorContainsOnlyCoordinateAndPane(t *testing.T) {
	typeOfCursor := reflect.TypeFor[Cursor]()
	if typeOfCursor.NumField() != 2 || typeOfCursor.Field(0).Name != "Coordinate" || typeOfCursor.Field(1).Name != "Pane" {
		t.Fatalf("Cursor fields = %#v", reflect.VisibleFields(typeOfCursor))
	}
}

func TestUnifiedNavigationSearchAndFileJumps(t *testing.T) {
	v := NewUnifiedView(testPatch(), true)
	first, ok := v.First()
	if !ok {
		t.Fatal("First returned no cursor")
	}
	line, _ := v.Line(first)
	if line.Text != "before" {
		t.Fatalf("first line = %q", line.Text)
	}
	next, ok := v.Move(first, Forward)
	if !ok {
		t.Fatal("Move returned no cursor")
	}
	line, _ = v.Line(next)
	if line.Kind != patch.Deletion {
		t.Fatalf("next kind = %v", line.Kind)
	}
	match, ok := v.Search("added", first, Forward)
	if !ok {
		t.Fatal("Search returned no cursor")
	}
	line, _ = v.Line(match)
	if line.Text != "added one" {
		t.Fatalf("match = %q", line.Text)
	}
	jumped, ok := v.JumpFile(first, Forward)
	if !ok {
		t.Fatal("JumpFile returned no cursor")
	}
	file, _ := v.File(jumped)
	if file.DisplayPath != "second.go" {
		t.Fatalf("jumped file = %q", file.DisplayPath)
	}
	if _, ok := v.JumpFile(jumped, Forward); ok {
		t.Fatal("JumpFile wrapped unexpectedly")
	}
}

func TestSplitPairsChangeBlocksAndSupportsEmptyPanes(t *testing.T) {
	v := NewSideBySideView(testPatch(), true)
	first, _ := v.First()
	removed, _ := v.Move(first, Forward)
	if removed.Pane != Right {
		t.Fatalf("initial pane = %v", removed.Pane)
	}
	removed, ok := v.SwitchPane(removed, Left)
	if !ok {
		t.Fatal("could not switch to deletion pane")
	}
	line, _ := v.Line(removed)
	if line.Text != "removed one" {
		t.Fatalf("left line = %q", line.Text)
	}
	added, ok := v.SwitchPane(removed, Right)
	if !ok {
		t.Fatal("paired addition missing")
	}
	line, _ = v.Line(added)
	if line.Text != "added one" {
		t.Fatalf("right line = %q", line.Text)
	}
	secondRemoved, _ := v.Move(removed, Forward)
	if _, ok := v.SwitchPane(secondRemoved, Right); !ok {
		t.Fatal("pane switch should find a nearby right line")
	}

	viewport := v.NewViewport(100, 20)
	rendered := v.Render(viewport, added, nil)
	if !strings.Contains(rendered, "removed one") || !strings.Contains(rendered, "added one") {
		t.Fatalf("paired render missing lines: %q", rendered)
	}
	for _, row := range strings.Split(rendered, "\n") {
		if strings.Contains(row, " │ ") && lipgloss.Width(row) != 100 {
			t.Fatalf("rendered width = %d", lipgloss.Width(row))
		}
	}
}

func TestSelectionLinesAndAnchor(t *testing.T) {
	v := NewUnifiedView(testPatch(), true)
	first, _ := v.First()
	last, _ := v.Move(first, Forward)
	last, _ = v.Move(last, Forward)
	selection := v.BeginSelection(first)
	selection, ok := v.ExtendSelection(selection, last)
	if !ok {
		t.Fatal("selection extension failed")
	}
	lines := v.Lines(selection)
	if len(lines) != 3 {
		t.Fatalf("selected lines = %d", len(lines))
	}
	anchor, err := v.Anchor(selection)
	if err != nil {
		t.Fatal(err)
	}
	if anchor.FilePath != "first.go" {
		t.Fatalf("anchor = %#v", anchor)
	}
	if got := strings.Join(anchor.QuotedLines, "|"); got != " before|-removed one|-removed two" {
		t.Fatalf("quoted lines = %q", got)
	}

	nextFile, _ := v.JumpFile(first, Forward)
	if _, ok := v.ExtendSelection(selection, nextFile); ok {
		t.Fatal("selection crossed a hunk")
	}
}

func TestViewportAlignmentResizeAndScrolling(t *testing.T) {
	v := NewUnifiedView(longPatch(), true)
	first, _ := v.First()
	cursor := first
	for range 8 {
		cursor, _ = v.Move(cursor, Forward)
	}
	viewport := v.NewViewport(30, 5)
	viewport = v.KeepVisible(viewport, cursor)
	if cursor.Coordinate.Y < viewport.Top.Y || cursor.Coordinate.Y >= viewport.Top.Y+viewport.Height {
		t.Fatalf("cursor not visible: %#v %#v", cursor, viewport)
	}
	viewport = v.Align(viewport, cursor, Middle)
	headerHeight := 0
	if v.(*diffView).hasStickyHeader(viewport.Top, viewport.Height) {
		headerHeight = 1
	}
	if headerHeight+cursor.Coordinate.Y-viewport.Top.Y != viewport.Height/2 {
		t.Fatalf("middle alignment = %#v", viewport)
	}
	viewport = v.ScrollHorizontal(viewport, 4)
	if viewport.LeftColumn == 0 {
		t.Fatal("horizontal scroll did not move")
	}
	viewport = v.Resize(viewport, 20, 3)
	if viewport.Width != 20 || viewport.Height != 3 {
		t.Fatalf("resize = %#v", viewport)
	}
	before := cursor
	viewport, cursor = v.ScrollHalfPage(viewport, cursor, Forward)
	if cursor.Coordinate.Y < before.Coordinate.Y {
		t.Fatalf("half page moved backward: %#v -> %#v", before, cursor)
	}
}

func TestViewportProgressUsesVisibleBottom(t *testing.T) {
	v := NewUnifiedView(longPatch(), true)
	viewport := v.NewViewport(30, 5)
	if progress := v.ViewportProgress(viewport); progress <= 0 || progress >= 100 {
		t.Fatalf("initial progress=%d", progress)
	}
	last, _ := v.Last()
	viewport = v.KeepVisible(viewport, last)
	if progress := v.ViewportProgress(viewport); progress != 100 {
		t.Fatalf("final progress=%d", progress)
	}
}

func TestHalfPageScrollingMovesCursorToFileBoundaries(t *testing.T) {
	v := NewUnifiedView(longPatch(), true)
	first, _ := v.First()
	last, _ := v.Last()
	viewport := v.NewViewport(30, 5)
	cursor := first

	for range len(longPatch().Files[0].Hunks[0].Lines) {
		viewport, cursor = v.ScrollHalfPage(viewport, cursor, Forward)
	}
	if cursor != last {
		t.Fatalf("cursor after scrolling down = %#v, want %#v", cursor, last)
	}

	for range len(longPatch().Files[0].Hunks[0].Lines) {
		viewport, cursor = v.ScrollHalfPage(viewport, cursor, Backward)
	}
	if cursor != first {
		t.Fatalf("cursor after scrolling up = %#v, want %#v", cursor, first)
	}
}

func TestFindCursorUsesSemanticIdentityAcrossChangedCoordinates(t *testing.T) {
	original := testPatch()
	oldView := NewUnifiedView(original, true)
	cursor, _ := oldView.Search("added one", mustFirst(t, oldView), Forward)
	file, _ := oldView.File(cursor)
	hunk, _ := oldView.Hunk(cursor)
	line, _ := oldView.Line(cursor)
	changed := testPatch()
	changed.Files[0].Metadata = []string{"mode changed", "more metadata"}
	newView := NewUnifiedView(changed, true)
	translated, ok := newView.FindCursor(file, hunk, line, cursor.Coordinate, cursor.Pane)
	if !ok {
		t.Fatal("semantic cursor was not found")
	}
	if translated.Coordinate == cursor.Coordinate {
		t.Fatal("cursor coordinate was reused after rows shifted")
	}
	translatedLine, _ := newView.Line(translated)
	if translatedLine != line {
		t.Fatalf("translated line = %#v, want %#v", translatedLine, line)
	}
}

func TestSplitViewWithOnlyDeletionsStartsInLeftPane(t *testing.T) {
	p := patch.Patch{Files: []patch.File{{DisplayPath: "deleted.go", Hunks: []patch.Hunk{{Header: "@@", Lines: []patch.Line{{Kind: patch.Deletion, Text: "gone", OldNumber: 1}}}}}}}
	v := NewSideBySideView(p, true)
	cursor, ok := v.First()
	if !ok || cursor.Pane != Left {
		t.Fatalf("first cursor = %#v, %v", cursor, ok)
	}
	line, _ := v.Line(cursor)
	if line.Text != "gone" {
		t.Fatalf("first line = %q", line.Text)
	}
}

func TestFindCursorFallsBackNearRemovedLine(t *testing.T) {
	original := testPatch()
	oldView := NewUnifiedView(original, true)
	cursor, _ := oldView.Search("removed two", mustFirst(t, oldView), Forward)
	file, _ := oldView.File(cursor)
	hunk, _ := oldView.Hunk(cursor)
	line, _ := oldView.Line(cursor)
	changed := testPatch()
	changed.Files[0].Hunks[0].Lines = changed.Files[0].Hunks[0].Lines[:2]
	newView := NewUnifiedView(changed, true)
	fallback, ok := newView.FindCursor(file, hunk, line, cursor.Coordinate, cursor.Pane)
	if !ok {
		t.Fatal("nearby cursor was not found")
	}
	fallbackLine, _ := newView.Line(fallback)
	if fallbackLine.Kind != patch.Deletion {
		t.Fatalf("fallback kind = %v", fallbackLine.Kind)
	}
}

func mustFirst(t *testing.T, v View) Cursor {
	t.Helper()
	cursor, ok := v.First()
	if !ok {
		t.Fatal("no first cursor")
	}
	return cursor
}

func testPatch() patch.Patch {
	return patch.Patch{Repository: "/repo", Files: []patch.File{
		{DisplayPath: "first.go", OldPath: "first.go", NewPath: "first.go", Hunks: []patch.Hunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patch.Line{
			{Kind: patch.Context, Text: "before", OldNumber: 1, NewNumber: 1},
			{Kind: patch.Deletion, Text: "removed one", OldNumber: 2},
			{Kind: patch.Deletion, Text: "removed two", OldNumber: 3},
			{Kind: patch.Addition, Text: "added one", NewNumber: 2},
			{Kind: patch.Context, Text: "after", OldNumber: 4, NewNumber: 3},
		}}}},
		{DisplayPath: "second.go", OldPath: "second.go", NewPath: "second.go", Hunks: []patch.Hunk{{Header: "@@ -1 +1 @@", Lines: []patch.Line{{Kind: patch.Addition, Text: "other", NewNumber: 1}}}}},
	}}
}

func longPatch() patch.Patch {
	lines := make([]patch.Line, 20)
	for index := range lines {
		lines[index] = patch.Line{Kind: patch.Context, Text: strings.Repeat("long", 20), OldNumber: patch.LineNumber(index + 1), NewNumber: patch.LineNumber(index + 1)}
	}
	return patch.Patch{Files: []patch.File{{DisplayPath: "long.go", Hunks: []patch.Hunk{{Header: "@@", Lines: lines}}}}}
}
