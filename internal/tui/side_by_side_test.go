package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/eskelinenantti/review-my-slop/internal/review"
)

func TestVisualLayoutPairsUnequalChangeBlocks(t *testing.T) {
	rows := []row{
		codeRow(review.LineRemoved),
		codeRow(review.LineRemoved),
		codeRow(review.LineAdded),
		codeRow(review.LineAdded),
		codeRow(review.LineAdded),
		codeRow(review.LineContext),
	}

	layout := newVisualLayout(rows, true)

	if layout.len() != 4 {
		t.Fatalf("visual rows = %d, want 4", layout.len())
	}
	want := []visualRow{
		{source: 0, left: 0, right: 2},
		{source: 1, left: 1, right: 3},
		{source: 4, left: -1, right: 4},
		{source: 5, left: 5, right: 5},
	}
	for position, expected := range want {
		if actual := layout.row(position); actual != expected {
			t.Errorf("visual row %d = %#v, want %#v", position, actual, expected)
		}
	}
	for source, position := range []int{0, 1, 0, 1, 2, 3} {
		if actual := layout.position(source); actual != position {
			t.Errorf("source row %d maps to %d, want %d", source, actual, position)
		}
	}
}

func TestVisualLayoutDoesNotPairAcrossHunks(t *testing.T) {
	removed := codeRow(review.LineRemoved)
	added := codeRow(review.LineAdded)
	added.hunkIndex = 1

	layout := newVisualLayout([]row{removed, added}, true)

	if layout.len() != 2 {
		t.Fatalf("visual rows = %d, want 2 separate rows", layout.len())
	}
	if layout.row(0).right != -1 || layout.row(1).left != -1 {
		t.Fatalf("rows from different hunks were paired: %#v", layout.visual)
	}
}

func TestResizeAcrossSideBySideThresholdPreservesCursorScreenRow(t *testing.T) {
	model := New(testDiff(), nil, nil)
	model.width = 120
	model.height = 6
	model.sideBySide = true
	model.rows = []row{
		codeRow(review.LineRemoved),
		codeRow(review.LineAdded),
		codeRow(review.LineContext),
		codeRow(review.LineRemoved),
		codeRow(review.LineAdded),
		codeRow(review.LineContext),
	}
	model.cursor = 5
	model.viewportTop = 1

	model = update(t, model, tea.WindowSizeMsg{Width: 80, Height: 6})
	if model.viewportTop != 3 {
		t.Fatalf("unified viewport top = %d, want 3", model.viewportTop)
	}
	if screenRow := model.visualLayout().position(model.cursor) - model.viewportTop; screenRow != 2 {
		t.Fatalf("unified cursor screen row = %d, want 2", screenRow)
	}

	model = update(t, model, tea.WindowSizeMsg{Width: 120, Height: 6})
	if model.viewportTop != 1 {
		t.Fatalf("side-by-side viewport top = %d, want 1", model.viewportTop)
	}
	if screenRow := model.visualLayout().position(model.cursor) - model.viewportTop; screenRow != 2 {
		t.Fatalf("side-by-side cursor screen row = %d, want 2", screenRow)
	}
}

func codeRow(kind review.LineKind) row {
	return row{
		kind:      rowCode,
		fileIndex: 0,
		hunkIndex: 0,
		line:      review.Line{Kind: kind},
	}
}
