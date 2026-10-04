package diffscreen

import "testing"

func TestClickAndDragSelectCode(t *testing.T) {
	v := New(testPatch(), Options{})
	v.Resize(80, 20)
	if !v.BeginDrag(20, 4) {
		t.Fatal("code click was ignored")
	}
	_, line, _ := v.Current()
	if line.Text != "removed one" || v.selection != nil {
		t.Fatal("click did not focus code without selecting a range")
	}
	v.DragTo(30, 6)
	v.EndDrag()
	_, lines, ok := v.Selected()
	if !ok || len(lines) != 3 || lines[0].Text != "removed one" || lines[2].Text != "added one" {
		t.Fatalf("drag selection = %#v", lines)
	}
	end := v.cursor
	v.DragTo(20, 3)
	if v.cursor != end {
		t.Fatal("motion after release changed selection")
	}
	if !v.BeginDrag(20, 6) || v.selection != nil {
		t.Fatal("new click did not clear old selection")
	}
	v.DragTo(20, 3)
	_, lines, ok = v.Selected()
	if !ok || len(lines) != 4 {
		t.Fatalf("backward drag = %#v", lines)
	}
	v.DragTo(20, 6)
	_, lines, _ = v.Selected()
	if len(lines) != 1 {
		t.Fatal("drag back to origin did not shrink selection")
	}
}

func TestMouseHitTestingMatchesScrolledProjection(t *testing.T) {
	for _, split := range []bool{false, true} {
		v := New(longPatch(), Options{SideBySide: split})
		v.Resize(121, 10)
		v.ScrollVertical(5)
		v.ScrollHorizontal(20)
		for _, point := range [][2]int{{0, 0}, {0, 1}, {0, 8}, {0, 9}, {-1, 3}, {121, 3}} {
			if v.BeginDrag(point[0], point[1]) {
				t.Fatalf("non-code point %v accepted", point)
			}
		}
		if !v.BeginDrag(100, 2) || v.cursor.row != 5 || v.cursor.pane != right {
			t.Fatal("click did not account for viewport and sticky header")
		}
		v.DragTo(100, 4)
		_, lines, ok := v.Selected()
		if !ok || len(lines) != 3 {
			t.Fatalf("scrolled drag selection = %#v", lines)
		}
	}
}

func TestSplitMouseRejectsDividerEmptyPaneAndCrossPaneDrag(t *testing.T) {
	v := New(testPatch(), Options{SideBySide: true})
	v.Resize(120, 20)
	for _, x := range []int{58, 59, 60} {
		if v.BeginDrag(x, 3) {
			t.Fatal("divider accepted")
		}
	}
	if v.BeginDrag(80, 5) {
		t.Fatal("empty right pane accepted")
	}
	if !v.BeginDrag(10, 4) || v.cursor.pane != left {
		t.Fatal("deletion click did not focus old pane")
	}
	origin := v.cursor
	v.DragTo(80, 4)
	if v.cursor != origin || v.selection != nil {
		t.Fatal("drag crossed panes")
	}
	v.DragTo(10, 5)
	_, lines, ok := v.Selected()
	if !ok || len(lines) != 2 || lines[1].Text != "removed two" {
		t.Fatalf("old-pane drag = %#v", lines)
	}
}

func TestDragRejectsOtherHunksAndCancelsOnStateChanges(t *testing.T) {
	v := New(testPatch(), Options{})
	v.Resize(80, 20)
	v.BeginDrag(10, 3)
	v.DragTo(10, 4)
	cursor, selection := v.cursor, *v.selection
	v.DragTo(10, 10)
	if v.cursor != cursor || *v.selection != selection {
		t.Fatal("drag crossed file/hunk boundary")
	}
	for _, change := range []func(){
		func() { v.Update(testPatch()) },
		func() { v.Resize(80, 20) },
		func() { v.Move(NextLine) },
		func() { v.ClearSelection() },
	} {
		v.BeginDrag(10, 3)
		change()
		if v.drag != nil {
			t.Fatal("stale drag survived state change")
		}
	}
}
