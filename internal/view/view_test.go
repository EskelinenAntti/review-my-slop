package view

import (
	"fmt"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/review"
)

func TestUnifiedNavigationSearchAndFileJumps(t *testing.T) {
	v := NewUnifiedView(mixedChangePatch(), true)
	first := mustFirst(t, v)
	line := mustLine(t, v, first)
	if line.Text != "before" {
		t.Fatalf("first line = %q", line.Text)
	}
	next := mustMove(t, v, first, Forward)
	line = mustLine(t, v, next)
	if line.Kind != patch.Deletion {
		t.Fatalf("next kind = %v", line.Kind)
	}
	match := mustSearch(t, v, "added", first, Forward)
	line = mustLine(t, v, match)
	if line.Text != "added one" {
		t.Fatalf("match = %q", line.Text)
	}
	jumped := mustJumpFile(t, v, first, Forward)
	file := mustFile(t, v, jumped)
	if file.DisplayPath != "second.go" {
		t.Fatalf("jumped file = %q", file.DisplayPath)
	}
	if _, ok := v.JumpFile(jumped, Forward); ok {
		t.Fatal("JumpFile wrapped unexpectedly")
	}
}

func TestSplitPairsChangeBlocksAndSupportsEmptyPanes(t *testing.T) {
	v := NewSideBySideView(mixedChangePatch(), true)
	first := mustFirst(t, v)
	removed := mustMove(t, v, first, Forward)
	if removed.Pane != Right {
		t.Fatalf("initial pane = %v", removed.Pane)
	}
	removed, ok := v.SwitchPane(removed, Left)
	if !ok {
		t.Fatal("could not switch to deletion pane")
	}
	line := mustLine(t, v, removed)
	if line.Text != "removed one" {
		t.Fatalf("left line = %q", line.Text)
	}
	added, ok := v.SwitchPane(removed, Right)
	if !ok {
		t.Fatal("paired addition missing")
	}
	line = mustLine(t, v, added)
	if line.Text != "added one" {
		t.Fatalf("right line = %q", line.Text)
	}
	secondRemoved := mustMove(t, v, removed, Forward)
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
	v := NewUnifiedView(mixedChangePatch(), true)
	first := mustFirst(t, v)
	last := mustMove(t, v, first, Forward)
	last = mustMove(t, v, last, Forward)
	selection := v.BeginSelection(first)
	selection, ok := v.ExtendSelection(selection, last)
	if !ok {
		t.Fatal("selection extension failed")
	}
	lines := v.Lines(selection)
	if len(lines) != 3 {
		t.Fatalf("selected lines = %d", len(lines))
	}
	file := mustFile(t, v, first)
	anchor := review.NewAnchor(file.Path(), lines)
	if anchor.FilePath != "first.go" {
		t.Fatalf("anchor = %#v", anchor)
	}
	if got := strings.Join(anchor.QuotedLines, "|"); got != " before|-removed one|-removed two" {
		t.Fatalf("quoted lines = %q", got)
	}

	nextFile := mustJumpFile(t, v, first, Forward)
	if _, ok := v.ExtendSelection(selection, nextFile); ok {
		t.Fatal("selection crossed a hunk")
	}
}

func TestViewportAlignmentResizeAndScrolling(t *testing.T) {
	v := NewUnifiedView(manyLinePatch(), true)
	cursor := cursorAtLine(t, v, 9)
	viewport := v.NewViewport(30, 5)
	viewport = v.KeepVisible(viewport, cursor)
	if cursor.Coordinate.Y < viewport.Top.Y || cursor.Coordinate.Y >= viewport.Top.Y+viewport.Height {
		t.Fatalf("cursor not visible: %#v %#v", cursor, viewport)
	}
	viewport = v.Align(viewport, cursor, Middle)
	if cursor.Coordinate.Y < viewport.Top.Y || cursor.Coordinate.Y >= viewport.Top.Y+viewport.Height {
		t.Fatalf("middle alignment hid cursor: %#v", viewport)
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
	v := NewUnifiedView(manyLinePatch(), true)
	viewport := v.NewViewport(30, 5)
	if progress := v.ViewportProgress(viewport); progress <= 0 || progress >= 100 {
		t.Fatalf("initial progress=%d", progress)
	}
	last := mustLast(t, v)
	viewport = v.KeepVisible(viewport, last)
	if progress := v.ViewportProgress(viewport); progress != 100 {
		t.Fatalf("final progress=%d", progress)
	}
}

func TestHalfPageScrollingMovesCursorToFileBoundaries(t *testing.T) {
	v := NewUnifiedView(manyLinePatch(), true)
	first := mustFirst(t, v)
	last := mustLast(t, v)
	viewport := v.NewViewport(30, 5)
	cursor := first

	for cursor != last {
		before := cursor
		viewport, cursor = v.ScrollHalfPage(viewport, cursor, Forward)
		if cursor == before {
			t.Fatal("half-page scrolling stopped before the last line")
		}
	}
	if cursor != last {
		t.Fatalf("cursor after scrolling down = %#v, want %#v", cursor, last)
	}

	for cursor != first {
		before := cursor
		viewport, cursor = v.ScrollHalfPage(viewport, cursor, Backward)
		if cursor == before {
			t.Fatal("half-page scrolling stopped before the first line")
		}
	}
	if cursor != first {
		t.Fatalf("cursor after scrolling up = %#v, want %#v", cursor, first)
	}
}

func TestFindCursorUsesSemanticIdentityAcrossChangedCoordinates(t *testing.T) {
	original := mixedChangePatch()
	oldView := NewUnifiedView(original, true)
	cursor := mustSearch(t, oldView, "added one", mustFirst(t, oldView), Forward)
	file := mustFile(t, oldView, cursor)
	hunk := mustHunk(t, oldView, cursor)
	line := mustLine(t, oldView, cursor)
	changed := mixedChangePatch()
	changed.Files[0].Metadata = []string{"mode changed", "more metadata"}
	newView := NewUnifiedView(changed, true)
	translated, ok := newView.FindCursor(file, hunk, line, cursor.Coordinate, cursor.Pane)
	if !ok {
		t.Fatal("semantic cursor was not found")
	}
	if translated.Coordinate == cursor.Coordinate {
		t.Fatal("cursor coordinate was reused after rows shifted")
	}
	translatedLine := mustLine(t, newView, translated)
	if translatedLine != line {
		t.Fatalf("translated line = %#v, want %#v", translatedLine, line)
	}
}

func TestSplitViewWithOnlyDeletionsStartsInLeftPane(t *testing.T) {
	p := patch.Patch{Files: []patch.File{{
		DisplayPath: "deleted.go",
		Hunks: []patch.Hunk{{
			Header: "@@",
			Lines:  []patch.Line{{Kind: patch.Deletion, Text: "gone", OldNumber: 1}},
		}},
	}}}
	v := NewSideBySideView(p, true)
	cursor, ok := v.First()
	if !ok || cursor.Pane != Left {
		t.Fatalf("first cursor = %#v, %v", cursor, ok)
	}
	line := mustLine(t, v, cursor)
	if line.Text != "gone" {
		t.Fatalf("first line = %q", line.Text)
	}
}

func TestFindCursorFallsBackNearRemovedLine(t *testing.T) {
	original := mixedChangePatch()
	oldView := NewUnifiedView(original, true)
	cursor := mustSearch(t, oldView, "removed two", mustFirst(t, oldView), Forward)
	file := mustFile(t, oldView, cursor)
	hunk := mustHunk(t, oldView, cursor)
	line := mustLine(t, oldView, cursor)
	changed := mixedChangePatch()
	changed.Files[0].Hunks[0].Lines = changed.Files[0].Hunks[0].Lines[:2]
	newView := NewUnifiedView(changed, true)
	fallback, ok := newView.FindCursor(file, hunk, line, cursor.Coordinate, cursor.Pane)
	if !ok {
		t.Fatal("nearby cursor was not found")
	}
	fallbackLine := mustLine(t, newView, fallback)
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

func mustLast(t *testing.T, v View) Cursor {
	t.Helper()
	cursor, ok := v.Last()
	if !ok {
		t.Fatal("no last cursor")
	}
	return cursor
}

func mustMove(t *testing.T, v View, cursor Cursor, direction Direction) Cursor {
	t.Helper()
	next, ok := v.Move(cursor, direction)
	if !ok {
		t.Fatalf("could not move cursor %v from %#v", direction, cursor)
	}
	return next
}

func mustSearch(t *testing.T, v View, query string, cursor Cursor, direction Direction) Cursor {
	t.Helper()
	match, ok := v.Search(query, cursor, direction)
	if !ok {
		t.Fatalf("search %q returned no cursor", query)
	}
	return match
}

func mustJumpFile(t *testing.T, v View, cursor Cursor, direction Direction) Cursor {
	t.Helper()
	jumped, ok := v.JumpFile(cursor, direction)
	if !ok {
		t.Fatalf("could not jump to another file from %#v", cursor)
	}
	return jumped
}

func mustFile(t *testing.T, v View, cursor Cursor) patch.File {
	t.Helper()
	file, ok := v.File(cursor)
	if !ok {
		t.Fatalf("no file for cursor %#v", cursor)
	}
	return file
}

func mustHunk(t *testing.T, v View, cursor Cursor) patch.Hunk {
	t.Helper()
	hunk, ok := v.Hunk(cursor)
	if !ok {
		t.Fatalf("no hunk for cursor %#v", cursor)
	}
	return hunk
}

func mustLine(t *testing.T, v View, cursor Cursor) patch.Line {
	t.Helper()
	line, ok := v.Line(cursor)
	if !ok {
		t.Fatalf("no line for cursor %#v", cursor)
	}
	return line
}

func cursorAtLine(t *testing.T, v View, number patch.LineNumber) Cursor {
	t.Helper()
	cursor := mustFirst(t, v)
	for {
		line, ok := v.Line(cursor)
		if ok && line.NewNumber == number {
			return cursor
		}
		next, ok := v.Move(cursor, Forward)
		if !ok {
			break
		}
		cursor = next
	}
	t.Fatalf("line %d not found", number)
	return Cursor{}
}

func mixedChangePatch() patch.Patch {
	return patch.Patch{Repository: "/repo", Files: []patch.File{
		{
			DisplayPath: "first.go",
			OldPath:     "first.go",
			NewPath:     "first.go",
			Hunks: []patch.Hunk{{
				Header: "@@ -1,3 +1,3 @@",
				Lines: []patch.Line{
					{Kind: patch.Context, Text: "before", OldNumber: 1, NewNumber: 1},
					{Kind: patch.Deletion, Text: "removed one", OldNumber: 2},
					{Kind: patch.Deletion, Text: "removed two", OldNumber: 3},
					{Kind: patch.Addition, Text: "added one", NewNumber: 2},
					{Kind: patch.Context, Text: "after", OldNumber: 4, NewNumber: 3},
				},
			}},
		},
		{
			DisplayPath: "second.go",
			OldPath:     "second.go",
			NewPath:     "second.go",
			Hunks: []patch.Hunk{{
				Header: "@@ -1 +1 @@",
				Lines:  []patch.Line{{Kind: patch.Addition, Text: "other", NewNumber: 1}},
			}},
		},
	}}
}

func manyLinePatch() patch.Patch {
	lines := make([]patch.Line, 20)
	for index := range lines {
		lines[index] = patch.Line{
			Kind:      patch.Context,
			Text:      fmt.Sprintf("line %d %s", index+1, strings.Repeat("long", 20)),
			OldNumber: patch.LineNumber(index + 1),
			NewNumber: patch.LineNumber(index + 1),
		}
	}
	return patch.Patch{Files: []patch.File{
		{
			DisplayPath: "long.go",
			Hunks:       []patch.Hunk{{Header: "@@", Lines: lines}},
		},
	}}
}
