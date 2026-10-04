package navigation

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/eskelinenantti/review-my-slop/internal/layout"
	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

func navFixture() patch.Patch {
	return patch.Patch{Files: []patch.File{
		{OldPath: "one", NewPath: "one", DisplayPath: "one", Hunks: []patch.Hunk{{Header: "@@", Lines: []patch.Line{{Kind: patch.Context, Text: "a", OldNumber: 1, NewNumber: 1}, {Kind: patch.Deletion, Text: "old", OldNumber: 2}, {Kind: patch.Addition, Text: "new", NewNumber: 2}, {Kind: patch.Context, Text: "b", OldNumber: 3, NewNumber: 3}}}}},
		{OldPath: "two", NewPath: "two", DisplayPath: "two", Hunks: []patch.Hunk{{Header: "@@", Lines: []patch.Line{{Kind: patch.Context, Text: "c", OldNumber: 1, NewNumber: 1}}}}},
	}}
}

func TestNavigationSkipsEmptyPanesAndSelectionMovementIsAtomic(t *testing.T) {
	n := New(layout.Build(navFixture(), layout.Split), 80, 8)
	if n.cursor == nil || n.cursor.Pane != layout.Right {
		t.Fatalf("initial cursor = %#v", n.cursor)
	}
	n.Jump(layout.Cell{Row: 3, Pane: layout.Left})
	if n.cursor == nil || n.cursor.Pane != layout.Left {
		t.Fatal("jump to deletion pane failed")
	}
	n.BeginSelection()
	before := *n.cursor
	n.Jump(layout.Cell{Row: 8, Pane: layout.Left})
	if *n.cursor != before {
		t.Fatalf("invalid selection jump moved cursor: %#v", n.cursor)
	}
	n.CancelSelection()
	n.Move(layout.Forward)
	if n.cursor == nil || n.cursor.Row <= before.Row {
		t.Fatalf("move did not skip to next left line: %#v", n.cursor)
	}
}

func TestSnapshotIsDetachedAndReplacePreservesPosition(t *testing.T) {
	d := layout.Build(navFixture(), layout.Unified)
	n := New(d, 40, 4)
	cell, _ := d.Locate(patch.Position{File: 0, Hunk: 0, Line: 2}, layout.Right)
	n.Jump(cell)
	s := n.Snapshot()
	s.Cursor.Row = 0
	s.Selection = &layout.Selection{First: cell, Last: cell}
	s.Viewport.Top = 0
	if n.cursor.Row != cell.Row || n.selection != nil || n.viewport.Top == 0 {
		t.Fatal("snapshot mutation changed navigation")
	}
	changed := navFixture()
	changed.Files[0].Metadata = []string{"inserted"}
	n.Replace(layout.Build(changed, layout.Unified))
	preserved, ok := layoutPos(n)
	if !ok || preserved.Line != 2 {
		t.Fatalf("position after replacement = %#v, %v", preserved, ok)
	}
}

func TestReplaceUsesSemanticIdentityAcrossReorderingAndFallsBackWhenLineDisappears(t *testing.T) {
	original := navFixture()
	oldDoc := layout.Build(original, layout.Unified)
	n := New(oldDoc, 40, 4)
	wanted, _ := oldDoc.Locate(patch.Position{File: 0, Hunk: 0, Line: 2}, layout.Right)
	n.Jump(wanted)
	changed := navFixture()
	changed.Files[0], changed.Files[1] = changed.Files[1], changed.Files[0]
	changed.Files[1].Metadata = []string{"new metadata"}
	n.Replace(layout.Build(changed, layout.Unified))
	position, ok := layoutPos(n)
	if !ok || position.File != 1 || position.Line != 2 {
		t.Fatalf("position after file reorder = %#v, %v", position, ok)
	}

	changed.Files[1].Hunks[0].Lines = []patch.Line{
		{Kind: patch.Context, Text: "a", OldNumber: 1, NewNumber: 1},
		{Kind: patch.Addition, Text: "replacement", NewNumber: 2},
		{Kind: patch.Context, Text: "b", OldNumber: 3, NewNumber: 3},
	}
	n.Replace(layout.Build(changed, layout.Unified))
	position, ok = layoutPos(n)
	if !ok || position.File != 1 {
		t.Fatalf("fallback after line removal = %#v, %v", position, ok)
	}
}

func TestReplacePreservesCursorSelectionAndScreenRowAcrossLayoutChange(t *testing.T) {
	oldDoc := layout.Build(navFixture(), layout.Unified)
	n := New(oldDoc, 40, 5)
	first, _ := oldDoc.Locate(patch.Position{File: 0, Hunk: 0, Line: 1}, layout.Right)
	last, _ := oldDoc.Locate(patch.Position{File: 0, Hunk: 0, Line: 2}, layout.Right)
	n.Jump(first)
	n.BeginSelection()
	n.Jump(last)
	n.viewport.Top = last.Row - 1
	oldOffset := n.cursor.Row - n.viewport.Top
	n.Replace(layout.Build(navFixture(), layout.Split))
	s := n.Snapshot()
	if s.Cursor == nil || s.Selection == nil {
		t.Fatalf("replacement dropped logical state: %#v", s)
	}
	position, _ := n.doc.Position(*s.Cursor)
	if position.File != 0 || position.Hunk != 0 || position.Line != 2 {
		t.Fatalf("cursor after layout change = %#v", position)
	}
	if s.Cursor.Row-s.Viewport.Top != oldOffset {
		t.Fatalf("screen row changed from %d to %d", oldOffset, s.Cursor.Row-s.Viewport.Top)
	}
	if _, err := n.doc.Range(*s.Selection); err != nil {
		t.Fatalf("preserved selection invalid: %v", err)
	}
	firstPosition, _ := n.doc.Position(s.Selection.First)
	lastPosition, _ := n.doc.Position(s.Selection.Last)
	if firstPosition.Line != 1 || lastPosition.Line != 2 {
		t.Fatalf("selection endpoints after layout change = %#v, %#v", firstPosition, lastPosition)
	}
}

func TestFirstLastAndJumpFileRespectFileBoundaries(t *testing.T) {
	d := layout.Build(navFixture(), layout.Unified)
	n := New(d, 40, 4)
	first := n.Snapshot().Cursor
	if first == nil {
		t.Fatal("First returned nil")
	}
	firstPosition, _ := d.Position(*first)
	if firstPosition.File != 0 || firstPosition.Line != 0 {
		t.Fatalf("first position=%#v", firstPosition)
	}
	n.Last()
	last := n.Snapshot().Cursor
	lastPosition, _ := d.Position(*last)
	if lastPosition.File != 1 || lastPosition.Line != 0 {
		t.Fatalf("last position=%#v", lastPosition)
	}
	n.First()
	n.JumpFile(layout.Forward)
	position, _ := d.Position(*n.cursor)
	if position.File != 1 {
		t.Fatalf("jump file position=%#v", position)
	}
	before := *n.cursor
	n.BeginSelection()
	n.JumpFile(layout.Forward)
	if *n.cursor != before {
		t.Fatalf("jump past final file moved cursor: %#v", n.cursor)
	}
	if n.selection != nil {
		t.Fatal("file jump did not cancel selection when no next file existed")
	}
}

func TestSwitchPaneTranslatesBothSelectionEndpoints(t *testing.T) {
	d := layout.Build(navFixture(), layout.Split)
	n := New(d, 80, 8)
	first, _ := d.Locate(patch.Position{File: 0, Hunk: 0, Line: 1}, layout.Left)
	last, _ := d.Locate(patch.Position{File: 0, Hunk: 0, Line: 2}, layout.Left)
	n.Jump(first)
	n.BeginSelection()
	n.Jump(last)
	n.SwitchPane(layout.Right)
	snapshot := n.Snapshot()
	if snapshot.Cursor == nil || snapshot.Cursor.Pane != layout.Right || snapshot.Selection == nil {
		t.Fatalf("right pane state = %#v", snapshot)
	}
	if snapshot.Selection.First.Pane != layout.Right || snapshot.Selection.Last.Pane != layout.Right {
		t.Fatalf("selection endpoints were not both translated: %#v", snapshot.Selection)
	}
	if _, err := d.Range(*snapshot.Selection); err != nil {
		t.Fatalf("translated selection invalid: %v", err)
	}
}

func TestHalfPageInvalidSelectionDoesNotMoveViewport(t *testing.T) {
	d := layout.Build(navFixture(), layout.Unified)
	n := New(d, 40, 2)
	lastLine, _ := d.Locate(patch.Position{File: 0, Hunk: 0, Line: 3}, layout.Right)
	n.Jump(lastLine)
	n.BeginSelection()
	n.viewport.Top = lastLine.Row
	before := n.viewport
	n.HalfPage(layout.Forward)
	if n.viewport != before {
		t.Fatalf("viewport moved after rejected half page: before=%#v after=%#v", before, n.viewport)
	}
	if *n.cursor != lastLine {
		t.Fatalf("cursor moved after rejected half page: %#v", n.cursor)
	}
}

func TestEmptyDocumentAndViewportClamps(t *testing.T) {
	n := New(layout.Build(patch.Patch{}, layout.Unified), 0, 0)
	if n.Snapshot().Cursor != nil {
		t.Fatal("empty document has cursor")
	}
	n.Resize(20, 3)
	n.ScrollHorizontal(100)
	if n.viewport.Width != 20 || n.viewport.Height != 3 || n.viewport.LeftColumn != 0 {
		t.Fatalf("empty viewport = %#v", n.viewport)
	}
	n.Replace(layout.Build(navFixture(), layout.Unified))
	n.BeginSelection()
	n.Replace(layout.Build(patch.Patch{}, layout.Unified))
	if snapshot := n.Snapshot(); snapshot.Cursor != nil || snapshot.Selection != nil {
		t.Fatalf("replacement with empty document retained state: %#v", snapshot)
	}
}

func TestMovementAndPaneSwitchSkipEmptySides(t *testing.T) {
	p := patch.Patch{Files: []patch.File{{DisplayPath: "file", Hunks: []patch.Hunk{{Header: "@@", Lines: []patch.Line{
		{Kind: patch.Context, Text: "one", OldNumber: 1, NewNumber: 1},
		{Kind: patch.Addition, Text: "right only", NewNumber: 2},
		{Kind: patch.Context, Text: "two", OldNumber: 2, NewNumber: 3},
		{Kind: patch.Deletion, Text: "left only", OldNumber: 3},
		{Kind: patch.Context, Text: "three", OldNumber: 4, NewNumber: 4},
	}}}}}}
	d := layout.Build(p, layout.Split)
	n := New(d, 80, 5)
	n.Jump(layout.Cell{Row: 2, Pane: layout.Left})
	n.Move(layout.Forward)
	linePos, _ := n.doc.Position(*n.cursor)
	if linePos.Line != 2 {
		t.Fatalf("left pane skipped context, position=%#v", linePos)
	}
	n.SwitchPane(layout.Right)
	if n.cursor.Pane != layout.Right || n.cursor.Row != 4 {
		t.Fatalf("pane switch missed same row: %#v", n.cursor)
	}
	n.Move(layout.Forward)
	linePos, _ = n.doc.Position(*n.cursor)
	if linePos.Line != 4 {
		t.Fatalf("right pane did not skip left-only line: %#v", linePos)
	}
}

func TestFirstStartsInLeftPaneForPureDeletions(t *testing.T) {
	p := patch.Patch{Files: []patch.File{{DisplayPath: "deleted", Hunks: []patch.Hunk{{Header: "@@", Lines: []patch.Line{{Kind: patch.Deletion, Text: "gone", OldNumber: 1}}}}}}}
	n := New(layout.Build(p, layout.Split), 80, 5)
	if n.cursor == nil || n.cursor.Pane != layout.Left {
		t.Fatalf("initial cursor for deletion-only Patch = %#v", n.cursor)
	}
}

func TestSwitchPaneNoTargetLeavesCursorUnchanged(t *testing.T) {
	p := patch.Patch{Files: []patch.File{{DisplayPath: "added", Hunks: []patch.Hunk{{Header: "@@", Lines: []patch.Line{
		{Kind: patch.Addition, Text: "one", NewNumber: 1}, {Kind: patch.Addition, Text: "two", NewNumber: 2},
	}}}}}}
	n := New(layout.Build(p, layout.Split), 80, 5)
	before := *n.cursor
	n.SwitchPane(layout.Left)
	if *n.cursor != before {
		t.Fatalf("empty pane switch moved cursor: %#v", n.cursor)
	}
}

func TestSwitchPaneFindsNearestSelectableRowAboveOrBelow(t *testing.T) {
	p := patch.Patch{Files: []patch.File{{DisplayPath: "file", Hunks: []patch.Hunk{{Header: "@@", Lines: []patch.Line{
		{Kind: patch.Addition, Text: "right", NewNumber: 1},
		{Kind: patch.Context, Text: "both", OldNumber: 1, NewNumber: 2},
		{Kind: patch.Deletion, Text: "left", OldNumber: 2},
	}}}}}}
	d := layout.Build(p, layout.Split)
	n := New(d, 80, 5)
	first, _ := d.Locate(patch.Position{File: 0, Hunk: 0, Line: 0}, layout.Right)
	n.Jump(first)
	n.SwitchPane(layout.Left)
	position, _ := d.Position(*n.cursor)
	if position.Line != 1 {
		t.Fatalf("switch below addition-only row = %#v", position)
	}
	last, _ := d.Locate(patch.Position{File: 0, Hunk: 0, Line: 2}, layout.Left)
	n.Jump(last)
	n.SwitchPane(layout.Right)
	position, _ = d.Position(*n.cursor)
	if position.Line != 1 {
		t.Fatalf("switch above deletion-only row = %#v", position)
	}
}

func TestViewportAlignmentStickyRowsProgressAndHorizontalClamp(t *testing.T) {
	lines := make([]patch.Line, 20)
	for i := range lines {
		lines[i] = patch.Line{Kind: patch.Context, Text: strings.Repeat("long", 20), OldNumber: patch.LineNumber(i + 1), NewNumber: patch.LineNumber(i + 1)}
	}
	p := patch.Patch{Files: []patch.File{{DisplayPath: "long.go", Hunks: []patch.Hunk{{Header: "@@", Lines: lines}}}}}
	d := layout.Build(p, layout.Unified)
	n := New(d, 30, 5)
	if progress := n.Progress(); progress <= 0 || progress >= 100 {
		t.Fatalf("initial progress=%d", progress)
	}
	last, _ := d.Locate(patch.Position{File: 0, Hunk: 0, Line: 19}, layout.Right)
	n.Jump(last)
	if n.Progress() != 100 {
		t.Fatalf("last visible progress=%d", n.Progress())
	}
	if last.Row >= n.viewport.Top+n.contentHeight(n.viewport) {
		t.Fatalf("last cursor hidden by sticky header: %#v", n.viewport)
	}
	n.Align(Middle)
	if n.hasSticky(n.viewport.Top, n.viewport.Height) {
		if n.viewport.Top+1+n.viewport.Height/2 != last.Row {
			t.Fatalf("middle alignment with sticky header = %#v cursor=%#v", n.viewport, last)
		}
	} else if n.viewport.Top+n.viewport.Height/2 != last.Row {
		t.Fatalf("middle alignment = %#v cursor=%#v", n.viewport, last)
	}
	n.ScrollHorizontal(10000)
	if n.viewport.LeftColumn <= 0 {
		t.Fatal("long line did not permit horizontal scrolling")
	}
	n.ScrollHorizontal(-10000)
	if n.viewport.LeftColumn != 0 {
		t.Fatalf("horizontal start clamp=%d", n.viewport.LeftColumn)
	}
	n.Resize(20, 3)
	if n.viewport.Width != 20 || n.viewport.Height != 3 {
		t.Fatalf("resized viewport = %#v", n.viewport)
	}
	if last.Row < n.viewport.Top || last.Row >= n.viewport.Top+n.contentHeight(n.viewport) {
		t.Fatalf("resize hid cursor: %#v cursor=%#v", n.viewport, last)
	}

	n.viewport.Top = 4
	n.viewport = n.clamp(n.viewport)
	if !n.hasSticky(n.viewport.Top, n.viewport.Height) || n.contentHeight(n.viewport) != n.viewport.Height-1 {
		t.Fatalf("sticky header not counted in viewport: %#v", n.viewport)
	}
}

func TestUnicodeHorizontalLimitMatchesTerminalCellWidth(t *testing.T) {
	text := strings.Repeat("界e\u0301👩‍👩‍👧‍👦", 3) + "\tend"
	p := patch.Patch{Files: []patch.File{{DisplayPath: "unicode", Hunks: []patch.Hunk{{Header: "@@", Lines: []patch.Line{{Kind: patch.Context, Text: text, OldNumber: 1, NewNumber: 1}}}}}}}
	n := New(layout.Build(p, layout.Unified), 22, 1)
	expected := max(0, ansi.StringWidth(strings.ReplaceAll(text, "\t", "    "))-8)
	n.ScrollHorizontal(10000)
	if n.viewport.LeftColumn != expected {
		t.Fatalf("Unicode horizontal limit=%d, terminal cell width expects %d", n.viewport.LeftColumn, expected)
	}
}

func TestHalfPageUsesPairedVisualRowsInSplitLayout(t *testing.T) {
	lines := []patch.Line{
		{Kind: patch.Deletion, Text: "d1", OldNumber: 1}, {Kind: patch.Addition, Text: "a1", NewNumber: 1},
		{Kind: patch.Context, Text: "c1", OldNumber: 2, NewNumber: 2},
		{Kind: patch.Deletion, Text: "d2", OldNumber: 3}, {Kind: patch.Addition, Text: "a2", NewNumber: 3},
		{Kind: patch.Context, Text: "c2", OldNumber: 4, NewNumber: 4},
	}
	p := patch.Patch{Files: []patch.File{{DisplayPath: "paired", Hunks: []patch.Hunk{{Header: "@@", Lines: lines}}}}}
	d := layout.Build(p, layout.Split)
	n := New(d, 120, 4)
	start := *n.cursor
	n.HalfPage(layout.Forward)
	if n.cursor == nil || n.cursor.Row <= start.Row {
		t.Fatalf("half page did not move through split visual rows: %#v", n.Snapshot())
	}
}

func TestHalfPageMovesByVisualRowsAndReturnsAcrossFileBoundary(t *testing.T) {
	lines := make([]patch.Line, 12)
	for i := range lines {
		lines[i] = patch.Line{Kind: patch.Context, Text: "line", OldNumber: patch.LineNumber(i + 1), NewNumber: patch.LineNumber(i + 1)}
	}
	p := patch.Patch{Files: []patch.File{{DisplayPath: "one", Hunks: []patch.Hunk{{Header: "@@", Lines: lines}}}, {DisplayPath: "two", Hunks: []patch.Hunk{{Header: "@@", Lines: []patch.Line{{Kind: patch.Context, Text: "last", OldNumber: 1, NewNumber: 1}}}}}}}
	d := layout.Build(p, layout.Unified)
	n := New(d, 40, 4)
	initial := n.Snapshot()
	n.HalfPage(layout.Forward)
	forward := n.Snapshot()
	if forward.Cursor == nil || forward.Cursor.Row <= initial.Cursor.Row || forward.Viewport.Top <= initial.Viewport.Top {
		t.Fatalf("forward half page = %#v", forward)
	}
	n.HalfPage(layout.Backward)
	back := n.Snapshot()
	if back.Cursor == nil || back.Cursor.Row != initial.Cursor.Row || back.Viewport.Top != initial.Viewport.Top {
		t.Fatalf("half page round trip = %#v, initial %#v", back, initial)
	}
	last, _ := d.Locate(patch.Position{File: 1, Hunk: 0, Line: 0}, layout.Right)
	n.Jump(last)
	n.HalfPage(layout.Forward)
	if *n.cursor != last {
		t.Fatalf("half page moved beyond final file: %#v", n.cursor)
	}
}

func layoutPos(n *Navigation) (patch.Position, bool) {
	s := n.Snapshot()
	if s.Cursor == nil {
		return patch.Position{}, false
	}
	return n.doc.Position(*s.Cursor)
}
