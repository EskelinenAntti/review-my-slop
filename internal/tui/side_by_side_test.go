package tui

import (
	"reflect"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/eskelinenantti/review-my-slop/internal/review"
)

func TestSideBySideProjectionPairsUnequalChangeBlocks(t *testing.T) {
	rows := codeRows(
		review.LineRemoved,
		review.LineRemoved,
		review.LineAdded,
		review.LineAdded,
		review.LineAdded,
		review.LineContext,
	)

	projection := newSideBySideProjection(rows)
	want := []*sideBySideRow{
		{primaryRow: rowAfter(rows.first, 0), leftRow: rowAfter(rows.first, 0), rightRow: rowAfter(rows.first, 2)},
		{primaryRow: rowAfter(rows.first, 1), leftRow: rowAfter(rows.first, 1), rightRow: rowAfter(rows.first, 3)},
		{primaryRow: rowAfter(rows.first, 4), rightRow: rowAfter(rows.first, 4)},
		{primaryRow: rowAfter(rows.first, 5), leftRow: rowAfter(rows.first, 5), rightRow: rowAfter(rows.first, 5)},
	}
	if len(projection.sideBySideRows) != len(want) {
		t.Fatalf("side-by-side row count = %d, want %d", len(projection.sideBySideRows), len(want))
	}
	for count, expected := range want {
		actual := projection.sideBySideRows[count]
		if actual.primaryRow != expected.primaryRow || actual.leftRow != expected.leftRow || actual.rightRow != expected.rightRow {
			t.Errorf("side-by-side row %d = %#v, want %#v", count, actual, expected)
		}
	}
	for source := range rows.all() {
		mapped, ok := projection.displayedRowForSource(source)
		if !ok {
			t.Fatalf("source row %#v has no side-by-side row", source)
		}
		if mapped.sourceRow() == nil {
			t.Fatalf("source row %#v maps to an empty side-by-side row", source)
		}
	}
}

func TestSideBySideProjectionDoesNotPairAcrossHunks(t *testing.T) {
	removed := codeRow(review.LineRemoved)
	added := codeRow(review.LineAdded)
	added.hunk = &review.Hunk{Header: "other"}
	rows := rowsFrom(removed, added)

	projection := newSideBySideProjection(rows)

	if len(projection.sideBySideRows) != 2 {
		t.Fatalf("side-by-side row count = %d, want 2", len(projection.sideBySideRows))
	}
	if projection.sideBySideRows[0].rightRow != nil || projection.sideBySideRows[1].leftRow != nil {
		t.Fatalf("rows from different hunks were paired: %#v", projection.sideBySideRows)
	}
}

func TestSideBySideSelectionOnlyIncludesActivePane(t *testing.T) {
	model := New(testDiff(), nil, nil)
	model.width = 120
	model.sideBySide = true
	model.activePane = paneRight
	model.rows = codeRows(
		review.LineContext,
		review.LineRemoved,
		review.LineRemoved,
		review.LineAdded,
		review.LineAdded,
		review.LineContext,
	)
	for current, text := model.rows.first, "context"; current != nil; current = current.next {
		current.line.Text = text
		if current.line.Kind == review.LineAdded || current.line.Kind == review.LineContext {
			current.line.NewNumber++
		}
	}
	model.selectFrom = model.rows.first
	model.cursor = rowAfter(model.rows.first, 4)
	model.selecting = true

	removed := rowAfter(model.rows.first, 1)
	added := rowAfter(model.rows.first, 3)
	if model.selectedInPane(removed, paneLeft) {
		t.Fatal("left-pane removed row is selected by a right-pane selection")
	}
	if !model.selectedInPane(added, paneRight) {
		t.Fatal("right-pane added row is not selected")
	}
	if model.selectedInPane(model.selectFrom, paneLeft) {
		t.Fatal("shared context row is selected in the inactive pane")
	}
	if !model.selectedInPane(model.selectFrom, paneRight) {
		t.Fatal("shared context row is not selected in the active pane")
	}

	anchor, err := model.currentAnchor()
	if err != nil {
		t.Fatalf("currentAnchor() error = %v", err)
	}
	if got, want := anchor.QuotedLines, []string{" context", "+context", "+context"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("quoted lines = %#v, want %#v", got, want)
	}
}

func TestResizeAcrossSideBySideThresholdPreservesCursorScreenRow(t *testing.T) {
	model := New(testDiff(), nil, nil)
	model.width = 120
	model.height = 6
	model.sideBySide = true
	model.rows = codeRows(
		review.LineRemoved,
		review.LineAdded,
		review.LineContext,
		review.LineRemoved,
		review.LineAdded,
		review.LineContext,
	)
	model.cursor = rowAfter(model.rows.first, 5)
	model.viewportTop = rowAfter(model.rows.first, 2)

	model = update(t, model, tea.WindowSizeMsg{Width: 80, Height: 6})
	if got := countDisplayedRowsBetween(model.layout(), model.viewportTop, model.cursor); got != 2 {
		t.Fatalf("stacked cursor rows above = %d, want 2", got)
	}

	model = update(t, model, tea.WindowSizeMsg{Width: 120, Height: 6})
	if got := countDisplayedRowsBetween(model.layout(), model.viewportTop, model.cursor); got != 2 {
		t.Fatalf("side-by-side cursor rows above = %d, want 2", got)
	}
}

var codeRowFile = &review.File{Display: "file"}
var codeRowHunk = &review.Hunk{Header: "hunk"}

func codeRow(kind review.LineKind) *row {
	line := &review.Line{Kind: kind}
	return &row{kind: rowCode, file: codeRowFile, hunk: codeRowHunk, line: line}
}

func codeRows(kinds ...review.LineKind) rowList {
	var rows rowList
	for _, kind := range kinds {
		rows.append(codeRow(kind))
	}
	return rows
}

func rowsFrom(sourceRows ...*row) rowList {
	var rows rowList
	for _, current := range sourceRows {
		rows.append(current)
	}
	return rows
}

func rowAfter(current *row, count int) *row {
	for current != nil && count > 0 {
		current = current.next
		count--
	}
	return current
}
