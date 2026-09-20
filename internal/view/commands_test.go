package view

import "testing"

func TestNavigateMovesAndExtendsSelection(t *testing.T) {
	v := NewUnifiedView(testPatch(), true)
	first, ok := v.First()
	if !ok {
		t.Fatal("First returned no cursor")
	}
	selection := v.BeginSelection(first)
	state := State{Cursor: &first, Selection: &selection, Viewport: v.NewViewport(80, 12)}

	next, outcome := Navigate(v, state, Move(Forward))
	if outcome != NoOutcome {
		t.Fatalf("outcome = %v", outcome)
	}
	if next.Cursor == nil || next.Selection == nil {
		t.Fatalf("state = %#v", next)
	}
	if next.Selection.Last != *next.Cursor {
		t.Fatalf("selection end = %#v, cursor = %#v", next.Selection.Last, *next.Cursor)
	}
	if next.Viewport.Top.Y > next.Cursor.Coordinate.Y {
		t.Fatalf("cursor %v is above viewport %#v", *next.Cursor, next.Viewport)
	}
}

func TestNavigateSearchReportsNoMatchWithoutChangingState(t *testing.T) {
	v := NewUnifiedView(testPatch(), true)
	first, _ := v.First()
	state := State{Cursor: &first, Viewport: v.NewViewport(80, 12)}

	next, outcome := Navigate(v, state, Search("missing", Forward))
	if outcome != NoMatch {
		t.Fatalf("outcome = %v", outcome)
	}
	if next.Cursor == nil || *next.Cursor != first || next.Viewport != state.Viewport {
		t.Fatalf("state changed: %#v", next)
	}
}

func TestNavigateJumpFileClearsSelection(t *testing.T) {
	v := NewUnifiedView(testPatch(), true)
	first, _ := v.First()
	selection := v.BeginSelection(first)
	state := State{Cursor: &first, Selection: &selection, Viewport: v.NewViewport(80, 12)}

	next, outcome := Navigate(v, state, JumpFile(Forward))
	if outcome != NoOutcome {
		t.Fatalf("outcome = %v", outcome)
	}
	if next.Selection != nil {
		t.Fatalf("selection = %#v", next.Selection)
	}
	if next.Cursor == nil || next.Cursor.Coordinate == first.Coordinate {
		t.Fatalf("cursor = %#v", next.Cursor)
	}
}

func TestNavigateScrollDoesNotMoveCursorOrSelection(t *testing.T) {
	v := NewUnifiedView(testPatch(), true)
	first, _ := v.First()
	selection := v.BeginSelection(first)
	state := State{Cursor: &first, Selection: &selection, Viewport: v.NewViewport(20, 12)}

	next, outcome := Navigate(v, state, ScrollColumns(4))
	if outcome != NoOutcome {
		t.Fatalf("outcome = %v", outcome)
	}
	if next.Cursor == nil || *next.Cursor != first || next.Selection != state.Selection {
		t.Fatalf("cursor or selection changed: %#v", next)
	}
	if next.Viewport.LeftColumn == state.Viewport.LeftColumn {
		t.Fatal("horizontal scroll did not change viewport")
	}
}
